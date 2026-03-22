package stun

import "errors"

var (
	// ErrAllProbesFailed indicates that every STUN server probe failed.
	ErrAllProbesFailed = errors.New("stun: all probes failed")

	// ErrTimeout indicates a STUN probe timed out waiting for a response.
	ErrTimeout = errors.New("stun: probe timed out")

	// ErrMalformedResponse indicates the STUN server returned an unparseable response.
	ErrMalformedResponse = errors.New("stun: malformed response")

	// ErrNoMappedAddress indicates the STUN response did not contain a mapped address.
	ErrNoMappedAddress = errors.New("stun: no mapped address in response")

	// ErrInsufficientProbes indicates too few probes succeeded to classify NAT type.
	ErrInsufficientProbes = errors.New("stun: insufficient successful probes for NAT classification")
)
