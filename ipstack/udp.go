package ipstack

import "net"

// A UDPPacket is a UDP payload contained in an IP packet.
type UDPPacket interface {
	// Valid verifies various fields of the packet.
	Valid() bool

	// SourceAddr gets the source IP and port.
	SourceAddr() *net.UDPAddr

	// DestAddr gets the destination IP and port.
	DestAddr() *net.UDPAddr

	// Header gets the raw UDP header as a slice into the
	// packet's data.
	Header() UDPHeader

	// Payload returns the contents of the packet as a slice
	// into the packet's contents.
	Payload() []byte

	// UseChecksum checks if the packet has checksums
	// disabled.
	// If true, Checksum() needn't be checked.
	UseChecksum() bool

	// Checksum computes the packet's checksum.
	// Zero means the checksum is correct.
	Checksum() uint16

	// SetChecksum computes the correct checksum and inserts
	// it into the packet.
	SetChecksum()
}

// A UDPHeader reads and writes fields of a UDP header.
type UDPHeader []byte

// SourcePort gets the packet's source port.
// This is optional and may be 0.
func (u UDPHeader) SourcePort() uint16 {
	return u.field(0)
}

// SourcePort sets the packet's source port.
func (u UDPHeader) SetSourcePort(val uint16) {
	u.setField(0, val)
}

// DestPort gets the packet's destination port.
func (u UDPHeader) DestPort() uint16 {
	return u.field(1)
}

// DestPort sets the packet's destination port.
func (u UDPHeader) SetDestPort(val uint16) {
	u.setField(1, val)
}

// Length gets the header's length field.
func (u UDPHeader) Length() uint16 {
	return u.field(2)
}

// Length sets the header's length field.
func (u UDPHeader) SetLength(val uint16) {
	u.setField(2, val)
}

// Checksum gets the header's checksum field.
// This is optional and may be 0.
func (u UDPHeader) Checksum() uint16 {
	return u.field(3)
}

// Checksum sets the header's checksum field.
func (u UDPHeader) SetChecksum(val uint16) {
	u.setField(3, val)
}

func (u UDPHeader) field(i int) uint16 {
	off := i << 1
	return (uint16(u[off]) << 8) | uint16(u[off+1])
}

func (u UDPHeader) setField(i int, val uint16) {
	off := i << 1
	u[off] = byte(val >> 8)
	u[off+1] = byte(val)
}

// A UDP4Packet is a UDP packet with an IPv4 header.
type UDP4Packet []byte

// Valid checks various invariants.
func (u UDP4Packet) Valid() bool {
	ipPacket := IPv4Packet(u)
	if !ipPacket.Valid() {
		return false
	}
	if len(ipPacket.Payload()) < 8 {
		return false
	}
	header := u.Header()
	return int(header.Length()) == len(header)+len(u.Payload())
}

// SourceAddr gets the source IP and port.
func (u UDP4Packet) SourceAddr() *net.UDPAddr {
	return &net.UDPAddr{
		IP:   IPv4Packet(u).SourceAddr(),
		Port: int(u.Header().SourcePort()),
	}
}

// DestAddr gets the destination IP and port.
func (u UDP4Packet) DestAddr() *net.UDPAddr {
	return &net.UDPAddr{
		IP:   IPv4Packet(u).DestAddr(),
		Port: int(u.Header().DestPort()),
	}
}

// Header gets the UDP header.
//
// This assumes that the packet is valid.
func (u UDP4Packet) Header() UDPHeader {
	return IPv4Packet(u).Payload()[:8]
}

// Payload gets the UDP payload.
//
// This assumes that the packet is valid.
func (u UDP4Packet) Payload() []byte {
	return IPv4Packet(u).Payload()[8:]
}

// TODO: implement checksums.
