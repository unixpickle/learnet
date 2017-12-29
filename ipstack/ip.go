package ipstack

import (
	"bytes"
	"net"
)

// IPv4Valid checks if the IP packet is valid.
func IPv4Valid(packet []byte) bool {
	if len(packet) < 20 {
		return false
	}
	if packet[0]>>4 != 4 {
		return false
	}
	size := int(packet[0]&0xf) * 4
	if size < 20 || size > len(packet) {
		return false
	}
	return true
}

// IPv4Header extracts the header from the IP packet.
//
// The result is a slice into the packet.
//
// The packet is assumed to be valid.
func IPv4Header(packet []byte) []byte {
	size := int(packet[0]&0xf) * 4
	return packet[:size]
}

// IPv4Payload extracts the payload from an IP packet.
//
// The result is a slice into the packet.
//
// The packet is assumed to be valid.
func IPv4Payload(packet []byte) []byte {
	return packet[len(IPv4Header(packet)):]
}

// IPv4SourceAddr extracts the source address from the IP
// packet.
//
// The result is a slice into the packet.
//
// The packet is assumed to be valid.
func IPv4SourceAddr(packet []byte) net.IP {
	return packet[12:16]
}

// IPv4DestAddr extracts the destination address from the
// IP packet.
//
// The result is a slice into the packet.
//
// The packet is assumed to be valid.
func IPv4DestAddr(packet []byte) net.IP {
	return packet[16:20]
}

// IPv4Proto extracts the protocol ID from the IP packet.
func IPv4Proto(packet []byte) int {
	return int(packet[9])
}

// IPv4Checksum computes the checksum of the data.
//
// For IPv4 packets, the data should be a header.
//
// A checksum of 0 is expected for valid data.
func IPv4Checksum(data []byte) uint16 {
	// Adapted from C example in RFC 1071:
	// https://tools.ietf.org/html/rfc1071.

	var sum uint32

	for len(data) >= 2 {
		sum += (uint32(data[0]) << 8) | uint32(data[1])
		data = data[2:]
	}

	if len(data) == 1 {
		sum += uint32(data[0])
	}

	for (sum >> 16) != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}

	return ^uint16(sum)
}

// IPv4SetChecksum inserts the checksum into a packet's
// header.
//
// The packet is assumed to be valid.
func IPv4SetChecksum(packet []byte) {
	header := IPv4Header(packet)
	header[10] = 0
	header[11] = 0
	checksum := IPv4Checksum(header)
	header[10] = byte(checksum >> 8)
	header[11] = byte(checksum)
}

// Filter IPv4 packets that are valid.
func FilterIPv4Valid(stream Stream) Stream {
	return Filter(stream, func(packet []byte) []byte {
		if IPv4Valid(packet) {
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
		if IPv4Checksum(IPv4Header(packet)) == 0 {
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
		if IPv4Proto(packet) == ipProto {
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
		if bytes.Equal(IPv4DestAddr(packet), dest[len(dest)-4:]) {
			return packet
		}
		return nil
	}, nil)
}
