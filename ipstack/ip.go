package ipstack

import (
	"bytes"
	"net"
	"sync"
)

// An IPv4Packet is a single packet intended to be sent or
// received on an IPv4 connection.
type IPv4Packet []byte

// NewIPv4Packet creates a valid IPv4Packet for the data.
func NewIPv4Packet(ttl, proto int, source, dest net.IP, payload []byte) IPv4Packet {
	res := make(IPv4Packet, 20+len(payload))
	res[0] = 0x45

	totalLen := uint16(len(payload) + 20)
	res[2] = byte(totalLen >> 8)
	res[3] = byte(totalLen)

	res.SetTTL(ttl)
	res.SetProto(proto)

	return res
}

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

// TTL extracts the time to live field from the packet.
func (i IPv4Packet) TTL() int {
	return int(i[8])
}

// SetTTL sets the time to live field for the packet.
func (i IPv4Packet) SetTTL(ttl int) {
	i[8] = byte(ttl)
}

// Proto extracts the protocol ID from the packet.
func (i IPv4Packet) Proto() int {
	return int(i[9])
}

// SetProto sets the protocol ID field.
func (i IPv4Packet) SetProto(proto int) {
	i[9] = byte(proto)
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

// Identification returns the packet's ID number.
//
// The packet is assumed to be valid.
func (i IPv4Packet) Identification() uint16 {
	return (uint16(i[4]) << 8) | uint16(i[5])
}

// SetIdentification updates the packet's ID number.
//
// The packet is assumed to be valid.
func (i IPv4Packet) SetIdentification(id uint16) {
	i[4] = byte(id >> 8)
	i[5] = byte(id)
}

// FragmentInfo returns the packet's fragment fields.
//
// The fragOffset value is measured in 8-byte blocks.
//
// The packet is assumed to be valid.
func (i IPv4Packet) FragmentInfo() (dontFrag, moreFrags bool, fragOffset int) {
	dontFrag = (i[6] & 0x40) != 0
	moreFrags = (i[6] & 0x20) != 0
	fragOffset = (int(i[6]&0x1f) << 8) | int(i[7])
	return
}

// SetFragmentInfo updates the packet's fragment fields.
//
// The packet is assumed to be valid.
func (i IPv4Packet) SetFragmentInfo(dontFrag, moreFrags bool, fragOffset int) {
	i[6] = 0
	i[7] = 0
	if dontFrag {
		i[6] |= 0x40
	}
	if moreFrags {
		i[6] |= 0x20
	}
	i[6] |= uint8(fragOffset>>8) & 0x1f
	i[7] |= uint8(fragOffset)
}

// FilterIPv4Valid filters packets that are valid.
func FilterIPv4Valid(stream Stream) Stream {
	return Filter(stream, func(packet []byte) []byte {
		if IPv4Packet(packet).Valid() {
			return packet
		}
		return nil
	}, nil)
}

// FilterIPv4Checksums drops packets with bad checksums.
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

// FilterIPv4Proto filters packets for a specific IP
// protocol.
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

// FilterIPv4Dest filters packets for a specific
// destination address.
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

// AddIPv4Identifiers assigns incrementing identification
// numbers to the outgoing packets.
//
// This should only be used if the outgoing packets are
// never going to be fragmented.
//
// All outgoing packets are assumed to be valid.
func AddIPv4Identifiers(stream Stream) Stream {
	var curId uint16
	var lock sync.Mutex
	return Filter(stream, nil, func(packet []byte) []byte {
		ipPacket := IPv4Packet(packet)
		lock.Lock()
		id := curId
		curId += 1
		lock.Unlock()
		_, moreFrags, offset := ipPacket.FragmentInfo()
		if moreFrags || offset != 0 {
			return nil
		}
		ipPacket.SetIdentification(id)
		ipPacket.SetChecksum()
		return packet
	})
}
