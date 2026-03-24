package transport

import (
	"testing"
)

func TestReliablePacketEncodeDecode(t *testing.T) {
	original := &reliablePacket{
		flags:   flagDATA | flagACK,
		seq:     42,
		ack:     41,
		payload: []byte("hello bore"),
	}

	encoded := encodePacket(original)

	decoded, err := decodePacket(encoded)
	if err != nil {
		t.Fatalf("decodePacket: %v", err)
	}

	if decoded.flags != original.flags {
		t.Errorf("flags: got %02x, want %02x", decoded.flags, original.flags)
	}
	if decoded.seq != original.seq {
		t.Errorf("seq: got %d, want %d", decoded.seq, original.seq)
	}
	if decoded.ack != original.ack {
		t.Errorf("ack: got %d, want %d", decoded.ack, original.ack)
	}
	if string(decoded.payload) != string(original.payload) {
		t.Errorf("payload: got %q, want %q", decoded.payload, original.payload)
	}
}

func TestReliablePacketEncodeDecodeEmpty(t *testing.T) {
	original := &reliablePacket{
		flags: flagACK,
		seq:   0,
		ack:   5,
	}

	encoded := encodePacket(original)
	decoded, err := decodePacket(encoded)
	if err != nil {
		t.Fatalf("decodePacket: %v", err)
	}

	if decoded.flags != flagACK {
		t.Errorf("flags: got %02x, want %02x", decoded.flags, flagACK)
	}
	if len(decoded.payload) != 0 {
		t.Errorf("payload should be empty, got %d bytes", len(decoded.payload))
	}
}

func TestReliablePacketDecodeTooShort(t *testing.T) {
	_, err := decodePacket([]byte{0x42, 0x4F})
	if err == nil {
		t.Error("expected error for short buffer")
	}
}

func TestReliablePacketDecodeBadMagic(t *testing.T) {
	buf := make([]byte, reliableHeaderLen)
	buf[0] = 0xFF // wrong magic
	_, err := decodePacket(buf)
	if err == nil {
		t.Error("expected error for bad magic")
	}
}

func TestReliablePacketSYNFIN(t *testing.T) {
	syn := &reliablePacket{flags: flagSYN, seq: 0}
	encoded := encodePacket(syn)
	decoded, err := decodePacket(encoded)
	if err != nil {
		t.Fatalf("decode SYN: %v", err)
	}
	if decoded.flags&flagSYN == 0 {
		t.Error("SYN flag not set")
	}

	fin := &reliablePacket{flags: flagFIN | flagACK, seq: 1, ack: 0}
	encoded = encodePacket(fin)
	decoded, err = decodePacket(encoded)
	if err != nil {
		t.Fatalf("decode FIN: %v", err)
	}
	if decoded.flags&flagFIN == 0 {
		t.Error("FIN flag not set")
	}
	if decoded.flags&flagACK == 0 {
		t.Error("ACK flag not set on FIN")
	}
}

func TestReliablePacketMaxPayload(t *testing.T) {
	payload := make([]byte, maxPayloadSize)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	pkt := &reliablePacket{
		flags:   flagDATA,
		seq:     1,
		payload: payload,
	}

	encoded := encodePacket(pkt)
	if len(encoded) != reliableHeaderLen+maxPayloadSize {
		t.Errorf("encoded length: got %d, want %d", len(encoded), reliableHeaderLen+maxPayloadSize)
	}

	decoded, err := decodePacket(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded.payload) != maxPayloadSize {
		t.Errorf("decoded payload length: got %d, want %d", len(decoded.payload), maxPayloadSize)
	}
	for i := range decoded.payload {
		if decoded.payload[i] != payload[i] {
			t.Errorf("payload byte %d: got %d, want %d", i, decoded.payload[i], payload[i])
			break
		}
	}
}

func TestReliableConstants(t *testing.T) {
	if reliableMagic != 0x424F5245 {
		t.Errorf("magic: got %08x, want 0x424F5245", reliableMagic)
	}
	if reliableHeaderLen != 15 {
		t.Errorf("header len: got %d, want 15", reliableHeaderLen)
	}
	if maxPayloadSize != 1200 {
		t.Errorf("max payload: got %d, want 1200", maxPayloadSize)
	}
}

func TestNewReliableConnNotNil(t *testing.T) {
	// We can't create a real net.Conn easily, but we can verify the constructor.
	// Use a mockConn through a type assertion—since ReliableConn takes net.Conn
	// and mockConn doesn't implement it, we test with nil and verify no panic
	// at construction time.
	rc := NewReliableConn(nil)
	if rc == nil {
		t.Fatal("NewReliableConn returned nil")
	}
}

// TestReliableConnSatisfiesConn is the compile-time check (already in reliable.go).
func TestReliableConnSatisfiesConn(t *testing.T) {
	var _ Conn = (*ReliableConn)(nil)
}
