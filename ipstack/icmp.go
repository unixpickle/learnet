package ipstack

const ProtocolNumberICMP = 1

const (
	ICMPTypeEchoReply   = 0
	ICMPTypeEchoRequest = 8
)

// An ICMPPacket is an ICMP datagram without an IP header.
type ICMPPacket []byte

// Valid checks for any obvious problems with the packet.
func (i ICMPPacket) Valid() bool {
	if len(i) < 8 {
		return false
	}
	return true
}

// Type extracts the ICMP type from the packet.
//
// The packet is assumed to be valid.
func (i ICMPPacket) Type() int {
	return int(i[0])
}

// SetType sets the ICMP type for the packet.
//
// The packet is assumed to be valid.
func (i ICMPPacket) SetType(t int) {
	i[0] = byte(t)
}

// Checksum computes the checksum of the packet.
//
// A checksum of 0 is expected.
func (i ICMPPacket) Checksum() uint16 {
	return InternetChecksum(i)
}

// SetChecksum inserts the correct checksum into the
// packet's header.
//
// The packet is assumed to be valid.
func (i ICMPPacket) SetChecksum() {
	i[2] = 0
	i[3] = 0
	checksum := i.Checksum()
	i[2] = byte(checksum >> 8)
	i[3] = byte(checksum)
}

// RespondToPingsIPv4 runs a loop that responds to pings
// on the stream.
//
// All incoming IPv4 packets are assumed to be valid.
//
// This returns when the stream is closed.
func RespondToPingsIPv4(stream Stream) {
	stream = FilterIPv4Proto(stream, ProtocolNumberICMP)

	for data := range stream.Incoming() {
		ipPacket := IPv4Packet(data)
		packet := ICMPPacket(ipPacket.Payload())
		if !packet.Valid() || packet.Checksum() != 0 || packet.Type() != ICMPTypeEchoRequest {
			continue
		}

		packet.SetType(ICMPTypeEchoReply)
		packet.SetChecksum()

		source := append([]byte{}, ipPacket.SourceAddr()...)
		copy(ipPacket.SourceAddr(), ipPacket.DestAddr())
		copy(ipPacket.DestAddr(), source)
		ipPacket.SetChecksum()

		Send(stream, ipPacket)
	}
}
