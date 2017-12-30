package ipstack

import (
	"bytes"
	"net"
)

// An IPv4Packet is a single packet intended to be sent or
// received on an IPv4 connection.
type IPv4Packet []byte

// Valid checks that various fields of the packet are
// correct or within range.
//
// This does not verify the checksum.
func (i IPv4Packet) Valid() bool {
	if len(i) < 20 {
		return false
	}
	if i[0]>>4 != 4 {
		return false
	}
	size := int(i[0]&0xf) * 4
	if size < 20 || size > len(i) {
		return false
	}
	return true
}

// Header extracts the header from the packet.
//
// The result is a slice into the packet.
//
// The packet is assumed to be valid.
func (i IPv4Packet) Header() []byte {
	size := int(i[0]&0xf) * 4
	return i[:size]
}

// Payload extracts the payload from a packet.
//
// The result is a slice into the packet.
//
// The packet is assumed to be valid.
func (i IPv4Packet) Payload() []byte {
	return i[len(i.Header()):]
}

// SourceAddr extracts the source address from the packet.
//
// The result is a slice into the packet.
//
// The packet is assumed to be valid.
func (i IPv4Packet) SourceAddr() net.IP {
	return net.IP(i[12:16])
}

// DestAddr extracts the destination address from the
// packet.
//
// The result is a slice into the packet.
//
// The packet is assumed to be valid.
func (i IPv4Packet) DestAddr() net.IP {
	return net.IP(i[16:20])
}

// Proto extracts the protocol ID from the packet.
func (i IPv4Packet) Proto() int {
	return int(i[9])
}

// Checksum computes the checksum of the header.
//
// A checksum of 0 is expected.
//
// The packet is assumed to be valid.
func (i IPv4Packet) Checksum() uint16 {
	return InternetChecksum(i.Header())
}

// SetChecksum inserts the correct checksum into the
// packet's header.
//
// The packet is assumed to be valid.
func (i IPv4Packet) SetChecksum() {
	i[10] = 0
	i[11] = 0
	checksum := i.Checksum()
	i[10] = byte(checksum >> 8)
	i[11] = byte(checksum)
}

// Filter IPv4 packets that are valid.
func FilterIPv4Valid(stream Stream) Stream {
	return Filter(stream, func(packet []byte) []byte {
		if IPv4Packet(packet).Valid() {
			return packet
		}
		return nil
	}, nil)
}

// Filter IPv4 packets with valid checksums.
//
// All incoming packets are assumed to be valid.
func FilterIPv4Checksums(stream Stream) Stream {
	return Filter(stream, func(packet []byte) []byte {
		if IPv4Packet(packet).Checksum() == 0 {
			return packet
		}
		return nil
	}, nil)
}

// Filter IPv4 packets for a given protocol.
//
// All incoming packets are assumed to be valid.
func FilterIPv4Proto(stream Stream, ipProto int) Stream {
	return Filter(stream, func(packet []byte) []byte {
		if IPv4Packet(packet).Proto() == ipProto {
			return packet
		}
		return nil
	}, nil)
}

// Filter incoming IPv4 packets for a destination address.
//
// All incoming packets are assumed to be valid.
func FilterIPv4Dest(stream Stream, dest net.IP) Stream {
	return Filter(stream, func(packet []byte) []byte {
		if bytes.Equal(IPv4Packet(packet).DestAddr(), dest[len(dest)-4:]) {
			return packet
		}
		return nil
	}, nil)
}
