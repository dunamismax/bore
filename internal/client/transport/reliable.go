// Package transport -- reliable UDP framing layer.
//
// This file implements a simple reliability layer over a connected UDP socket.
// The Noise handshake and bore transfer engine assume a stream (io.ReadWriter),
// so we need:
//
//  1. Message framing (length-prefix each application write)
//  2. Reliability (sequence numbers + selective ACK + retransmit)
//  3. Ordered delivery (reassemble in-sequence)
//
// The protocol is intentionally minimal -- just enough to carry bore's
// encrypted application frames over UDP with reasonable loss tolerance.
//
// Wire format per packet:
//
//	[4: magic 0x424F5245 "BORE"]
//	[1: flags]
//	[4: sequence number (BE)]
//	[4: ack number (BE)]
//	[2: payload length (BE)]
//	[N: payload]
//
// Total header: 15 bytes.
//
// Flags:
//
//	0x01 -- SYN  (connection open)
//	0x02 -- FIN  (connection close)
//	0x04 -- ACK  (acknowledgment, ack field is valid)
//	0x08 -- DATA (payload present)
package transport

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Protocol constants for the reliable UDP framing.
const (
	reliableMagic     = 0x424F5245 // "BORE"
	reliableHeaderLen = 15         // magic(4) + flags(1) + seq(4) + ack(4) + payloadLen(2)
	maxPayloadSize    = 1200       // conservative UDP MTU-safe payload
	maxPacketSize     = reliableHeaderLen + maxPayloadSize

	// retransmit and timeout defaults
	defaultRetransmitInterval = 200 * time.Millisecond
	defaultRetransmitMax      = 20
	defaultReadTimeout        = 30 * time.Second
)

// Flags
const (
	flagSYN  byte = 0x01
	flagFIN  byte = 0x02
	flagACK  byte = 0x04
	flagDATA byte = 0x08
)

var (
	errReliableClosed    = errors.New("reliable: connection closed")
	errReliableTimeout   = errors.New("reliable: operation timed out")
	errReliableBadPacket = errors.New("reliable: malformed packet")
)

// reliablePacket is a parsed packet on the wire.
type reliablePacket struct {
	flags   byte
	seq     uint32
	ack     uint32
	payload []byte
}

func encodePacket(p *reliablePacket) []byte {
	pLen := len(p.payload)
	buf := make([]byte, reliableHeaderLen+pLen)
	binary.BigEndian.PutUint32(buf[0:4], reliableMagic)
	buf[4] = p.flags
	binary.BigEndian.PutUint32(buf[5:9], p.seq)
	binary.BigEndian.PutUint32(buf[9:13], p.ack)
	binary.BigEndian.PutUint16(buf[13:15], uint16(pLen))
	if pLen > 0 {
		copy(buf[15:], p.payload)
	}
	return buf
}

func decodePacket(buf []byte) (*reliablePacket, error) {
	if len(buf) < reliableHeaderLen {
		return nil, errReliableBadPacket
	}
	if binary.BigEndian.Uint32(buf[0:4]) != reliableMagic {
		return nil, errReliableBadPacket
	}
	pLen := int(binary.BigEndian.Uint16(buf[13:15]))
	if len(buf) < reliableHeaderLen+pLen {
		return nil, errReliableBadPacket
	}
	p := &reliablePacket{
		flags: buf[4],
		seq:   binary.BigEndian.Uint32(buf[5:9]),
		ack:   binary.BigEndian.Uint32(buf[9:13]),
	}
	if pLen > 0 {
		p.payload = make([]byte, pLen)
		copy(p.payload, buf[15:15+pLen])
	}
	return p, nil
}

// ReliableConn wraps a net.Conn (connected UDP) to provide reliable, ordered
// delivery suitable for use as a bore transport Conn (io.ReadWriteCloser).
type ReliableConn struct {
	conn net.Conn

	// send state
	sendMu  sync.Mutex
	sendSeq uint32

	// recv state
	recvMu      sync.Mutex
	recvBuf     []byte // buffered reassembled data for Read
	recvPos     int
	expectedSeq uint32

	// ack tracking
	lastAcked uint32

	closed chan struct{}
	once   sync.Once
}

// NewReliableConn wraps a connected UDP net.Conn in the reliable framing layer.
func NewReliableConn(conn net.Conn) *ReliableConn {
	return &ReliableConn{
		conn:   conn,
		closed: make(chan struct{}),
	}
}

// Write sends data reliably. Each Write call becomes one or more DATA packets.
// Blocks until acknowledged or timeout.
func (c *ReliableConn) Write(p []byte) (int, error) {
	select {
	case <-c.closed:
		return 0, errReliableClosed
	default:
	}

	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	totalWritten := 0
	remaining := p

	for len(remaining) > 0 {
		chunkSize := len(remaining)
		if chunkSize > maxPayloadSize {
			chunkSize = maxPayloadSize
		}
		chunk := remaining[:chunkSize]
		remaining = remaining[chunkSize:]

		seq := c.sendSeq
		c.sendSeq++

		pkt := &reliablePacket{
			flags:   flagDATA | flagACK,
			seq:     seq,
			ack:     c.lastAcked,
			payload: chunk,
		}

		if err := c.sendWithRetransmit(pkt); err != nil {
			return totalWritten, err
		}

		totalWritten += chunkSize
	}

	return totalWritten, nil
}

// sendWithRetransmit sends a packet and waits for an ACK with retransmission.
func (c *ReliableConn) sendWithRetransmit(pkt *reliablePacket) error {
	encoded := encodePacket(pkt)
	ackCh := make(chan struct{}, 1)

	// Start a goroutine to watch for ACKs.
	go func() {
		buf := make([]byte, maxPacketSize)
		for {
			select {
			case <-c.closed:
				return
			default:
			}

			if err := c.conn.SetReadDeadline(time.Now().Add(defaultRetransmitInterval)); err != nil {
				return
			}
			n, err := c.conn.Read(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}

			resp, err := decodePacket(buf[:n])
			if err != nil {
				continue
			}

			// Process any data in the ACK packet (piggyback).
			if resp.flags&flagDATA != 0 && len(resp.payload) > 0 {
				c.bufferRecvData(resp.seq, resp.payload)
			}

			// Check if this ACKs our packet.
			if resp.flags&flagACK != 0 && resp.ack >= pkt.seq {
				select {
				case ackCh <- struct{}{}:
				default:
				}
				return
			}
		}
	}()

	for attempt := 0; attempt < defaultRetransmitMax; attempt++ {
		if _, err := c.conn.Write(encoded); err != nil {
			return fmt.Errorf("reliable send: %w", err)
		}

		select {
		case <-ackCh:
			return nil
		case <-time.After(defaultRetransmitInterval):
			continue
		case <-c.closed:
			return errReliableClosed
		}
	}

	return errReliableTimeout
}

// bufferRecvData appends received data to the reassembly buffer if it's the next expected sequence.
func (c *ReliableConn) bufferRecvData(seq uint32, data []byte) {
	c.recvMu.Lock()
	defer c.recvMu.Unlock()

	if seq == c.expectedSeq {
		c.recvBuf = append(c.recvBuf, data...)
		c.expectedSeq++
	}
	// Out-of-order packets are dropped (simple stop-and-wait). The retransmit
	// on the sender side will resend. For bore's transfer workload (sequential
	// encrypted chunks), this is sufficient.
}

// Read implements io.Reader. Returns buffered reassembled data or blocks
// waiting for the next DATA packet from the peer.
func (c *ReliableConn) Read(p []byte) (int, error) {
	// First drain any buffered data.
	c.recvMu.Lock()
	if c.recvPos < len(c.recvBuf) {
		n := copy(p, c.recvBuf[c.recvPos:])
		c.recvPos += n
		if c.recvPos >= len(c.recvBuf) {
			c.recvBuf = c.recvBuf[:0]
			c.recvPos = 0
		}
		c.recvMu.Unlock()
		return n, nil
	}
	c.recvMu.Unlock()

	// Block waiting for data from the peer.
	buf := make([]byte, maxPacketSize)
	for {
		select {
		case <-c.closed:
			return 0, io.EOF
		default:
		}

		if err := c.conn.SetReadDeadline(time.Now().Add(defaultReadTimeout)); err != nil {
			return 0, fmt.Errorf("reliable read deadline: %w", err)
		}

		n, err := c.conn.Read(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				select {
				case <-c.closed:
					return 0, io.EOF
				default:
					return 0, errReliableTimeout
				}
			}
			return 0, err
		}

		pkt, err := decodePacket(buf[:n])
		if err != nil {
			continue // skip malformed
		}

		// Handle FIN.
		if pkt.flags&flagFIN != 0 {
			// Send ACK for the FIN.
			ackPkt := &reliablePacket{
				flags: flagACK,
				ack:   pkt.seq,
			}
			c.conn.Write(encodePacket(ackPkt)) //nolint: errcheck
			return 0, io.EOF
		}

		// Send ACK for any received packet.
		if pkt.flags&flagDATA != 0 {
			c.lastAcked = pkt.seq
			ackPkt := &reliablePacket{
				flags: flagACK,
				seq:   c.sendSeq,
				ack:   pkt.seq,
			}
			c.conn.Write(encodePacket(ackPkt)) //nolint: errcheck

			if len(pkt.payload) > 0 {
				c.bufferRecvData(pkt.seq, pkt.payload)

				// Drain from buffer.
				c.recvMu.Lock()
				if c.recvPos < len(c.recvBuf) {
					copied := copy(p, c.recvBuf[c.recvPos:])
					c.recvPos += copied
					if c.recvPos >= len(c.recvBuf) {
						c.recvBuf = c.recvBuf[:0]
						c.recvPos = 0
					}
					c.recvMu.Unlock()
					return copied, nil
				}
				c.recvMu.Unlock()
			}
		}

		// Pure ACK packet -- no data for us. Keep reading.
	}
}

// Close sends a FIN and tears down the connection.
func (c *ReliableConn) Close() error {
	var err error
	c.once.Do(func() {
		close(c.closed)

		// Best-effort FIN.
		fin := &reliablePacket{
			flags: flagFIN | flagACK,
			seq:   c.sendSeq,
			ack:   c.lastAcked,
		}
		c.conn.Write(encodePacket(fin)) //nolint: errcheck
		err = c.conn.Close()
	})
	return err
}

// Verify ReliableConn satisfies Conn.
var _ Conn = (*ReliableConn)(nil)
