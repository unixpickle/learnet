package ipstack

const ProtocolNumberICMP = 1

const (
	ICMPTypeEchoReply   = 0
	ICMPTypeEchoRequest = 8
)

// ICMPChecksum computes the checksum of the ICMP packet.
//
// If the packet has a valid checksum, this is 0.
//
// The payload is assumed to be valid.
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
// This responds to pings in a synchronous manner, meaning
// that write backpressure can prevent reads.
// Thus, it is recommended that you use a write-dropping
// stream.
//
// This returns when the stream is closed.
func RespondToPingsIPv4(stream Stream) {
	stream = FilterIPv4Proto(stream, ProtocolNumberICMP)

	// TODO: ICMP checksum and validation filters.

	for packet := range stream.Incoming() {
		payload := IPv4Payload(packet)
		if payload[0] == ICMPTypeEchoRequest {
			source := append([]byte{}, IPv4SourceAddr(packet)...)
			copy(IPv4SourceAddr(packet), IPv4DestAddr(packet))
			copy(IPv4DestAddr(packet), source)
			payload[0] = ICMPTypeEchoReply
			ICMPSetChecksum(payload)
			IPv4SetChecksum(packet)
			select {
			case stream.Outgoing() <- packet:
			case <-stream.Done():
				return
			}
		}
	}
}
