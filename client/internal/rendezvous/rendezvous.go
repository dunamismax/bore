// Package rendezvous implements the bore send/receive coordination flows.
//
// Sender flow:
//  1. Dial the transport as sender -> receive session ID
//  2. Generate PAKE code (channel + words)
//  3. Compose full rendezvous code: room_id-channel-word-word-word
//  4. Notify caller with the code (via callback)
//  5. Wait for receiver; perform Noise XXpsk0 handshake; send file
//
// Receiver flow:
//  1. Parse full rendezvous code -> extract room ID and PAKE code
//  2. Dial the transport as receiver with the session ID
//  3. Perform Noise XXpsk0 handshake as responder; receive file
//
// The rendezvous layer is transport-agnostic: callers provide a
// [transport.Dialer] which may be a [transport.RelayDialer],
// [transport.DirectDialer], [transport.Selector], or any other
// implementation of the Dialer interface.
package rendezvous

import (
	"context"
	"fmt"

	"github.com/dunamismax/bore/client/internal/code"
	"github.com/dunamismax/bore/client/internal/crypto"
	"github.com/dunamismax/bore/client/internal/engine"
	"github.com/dunamismax/bore/client/internal/transport"
)

// DefaultRelayURL is the default bore relay server URL.
const DefaultRelayURL = "http://localhost:8080"

// SenderResult is returned from Send and SendWithCodeCallback.
type SenderResult struct {
	Code     code.FullRendezvousCode
	Transfer engine.SendResult
}

// ReceiverResult is returned from Receive.
type ReceiverResult struct {
	Transfer engine.ReceiveResult
}

// Send executes the full sender flow using the provided transport dialer.
//
//   - dialer: transport implementation (relay, direct, or selector)
//   - relayURL: relay server URL stored in the rendezvous code metadata
//   - filename: name of the file to send
//   - data: file contents
//   - wordCount: number of words in the PAKE code (2-5)
func Send(ctx context.Context, dialer transport.Dialer, relayURL, filename string, data []byte, wordCount int) (SenderResult, error) {
	return SendWithCodeCallback(ctx, dialer, relayURL, filename, data, wordCount, nil)
}

// SendWithCodeCallback executes the sender flow, calling onCode with the
// full rendezvous code before waiting for the receiver. The callback runs
// synchronously (the transport already holds the connection open) so the
// caller can display the code to the user before the handshake begins.
func SendWithCodeCallback(
	ctx context.Context,
	dialer transport.Dialer,
	relayURL, filename string,
	data []byte,
	wordCount int,
	onCode func(code.FullRendezvousCode),
) (SenderResult, error) {
	if relayURL == "" {
		relayURL = DefaultRelayURL
	}

	// Step 1: dial as sender, receive session ID (room ID for relay).
	sessionID, rw, err := dialer.DialSender(ctx)
	if err != nil {
		return SenderResult{}, fmt.Errorf("dial sender: %w", err)
	}

	// Step 2: generate PAKE code.
	pakeCode, err := code.Generate(wordCount)
	if err != nil {
		return SenderResult{}, fmt.Errorf("generate PAKE code: %w", err)
	}

	fullCode := code.FullRendezvousCode{
		RoomID:   sessionID,
		PakeCode: pakeCode,
		RelayURL: relayURL,
	}

	// Step 3: notify caller with the code before blocking on receiver.
	if onCode != nil {
		onCode(fullCode)
	}

	// Step 4: Noise handshake as initiator.
	pakeStr := pakeCode.String()
	ch, err := crypto.Handshake(crypto.Initiator, pakeStr, rw)
	if err != nil {
		return SenderResult{}, fmt.Errorf("handshake: %w", err)
	}

	// Step 5: send file.
	result, err := engine.SendData(ch, rw, filename, data)
	if err != nil {
		return SenderResult{}, fmt.Errorf("send data: %w", err)
	}

	return SenderResult{Code: fullCode, Transfer: result}, nil
}

// Receive executes the full receiver flow using the provided transport dialer.
//
//   - codeStr: the full rendezvous code from the sender
//   - dialer: transport implementation (relay, direct, or selector)
//   - relayURL: relay server URL for code parsing metadata
func Receive(ctx context.Context, codeStr string, dialer transport.Dialer, relayURL string) (ReceiverResult, error) {
	if relayURL == "" {
		relayURL = DefaultRelayURL
	}

	// Step 1: parse the rendezvous code.
	fullCode, err := code.ParseFull(codeStr, relayURL)
	if err != nil {
		return ReceiverResult{}, fmt.Errorf("parse rendezvous code: %w", err)
	}

	// Step 2: dial as receiver with the session ID (room ID for relay).
	rw, err := dialer.DialReceiver(ctx, fullCode.RoomID)
	if err != nil {
		return ReceiverResult{}, fmt.Errorf("dial receiver: %w", err)
	}

	// Step 3: Noise handshake as responder.
	pakeStr := fullCode.PakeCode.String()
	ch, err := crypto.Handshake(crypto.Responder, pakeStr, rw)
	if err != nil {
		return ReceiverResult{}, fmt.Errorf("handshake: %w", err)
	}

	// Step 4: receive file.
	result, err := engine.ReceiveData(ch, rw)
	if err != nil {
		return ReceiverResult{}, fmt.Errorf("receive data: %w", err)
	}

	return ReceiverResult{Transfer: result}, nil
}
