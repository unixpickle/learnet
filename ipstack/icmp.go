package ipstack

const ProtocolNumberICMP = 1

const (
	ICMPTypeEchoReply   = 0
	ICMPTypeEchoRequest = 8
)

// ICMPValid checks if an ICMP payload is valid.
func ICMPValid(payload []byte) bool {
	if len(payload) < 8 {
		return false
	}
	return true
}

// ICMPType extracts the ICMP type from the payload.
//
// The payload is assumed to be valid.
func ICMPType(payload []byte) int {
	return int(payload[0])
}

// ICMPChecksum computes the checksum of the ICMP payload.
//
// A checksum of 0 is expected.
func ICMPChecksum(payload []byte) uint16 {
	return IPv4Checksum(payload)
}

// ICMPSetChecksum inserts a checksum into an ICMP
// payload.
//
// The payload is assumed to be valid.
func ICMPSetChecksum(payload []byte) {
	payload[2] = 0
	payload[3] = 0
	checksum := ICMPChecksum(payload)
	payload[2] = byte(checksum >> 8)
	payload[3] = byte(checksum)
}

// RespondToPingsIPv4 runs a loop that responds to pings
// on the stream.
//
// This returns when the stream is closed.
func RespondToPingsIPv4(stream Stream) {
	stream = FilterIPv4Proto(stream, ProtocolNumberICMP)

	for packet := range stream.Incoming() {
		payload := IPv4Payload(packet)
		if !ICMPValid(payload) || ICMPChecksum(payload) != 0 ||
			ICMPType(payload) != ICMPTypeEchoRequest {
			continue
		}

		source := append([]byte{}, IPv4SourceAddr(packet)...)
		copy(IPv4SourceAddr(packet), IPv4DestAddr(packet))
		copy(IPv4DestAddr(packet), source)
		payload[0] = ICMPTypeEchoReply
		ICMPSetChecksum(payload)
		IPv4SetChecksum(packet)

		stream.Write(packet)
	}
}
