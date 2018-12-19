package ipstack

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"

	"github.com/unixpickle/essentials"
)

const ProtocolNumberTCP = 6

type Flag uint8

const (
	NS Flag = iota
	CWR
	ECE
	URG
	ACK
	PSH
	RST
	SYN
	FIN
)

// A TCPPacket is a TCP payload contained in an IP packet.
type TCPPacket interface {
	// Valid verifies various fields of the TCPPacket.
	Valid() bool

	// SourceAddr gets the source IP and port.
	SourceAddr() *net.TCPAddr

	// DestAddr gets the destination IP and port.
	DestAddr() *net.TCPAddr

	// Header gets the raw TCP TCPHeader as a slice into the
	// TCPPacket's data.
	Header() TCPHeader

	// Payload returns the contents of the packet as a slice
	// into the packet's contents.
	Payload() []byte

	// Checksum computes the packet's checksum.
	// Zero means the checksum is correct.
	Checksum() uint16

	// SetChecksum computes the correct checksum and inserts
	// it into the packet.
	SetChecksum()
}

// TCPHeader accesses fields of a TCP packet's header.
type TCPHeader []byte

func (t TCPHeader) SourcePort() uint16 {
	return binary.BigEndian.Uint16(t[:2])
}

func (t TCPHeader) SetSourcePort(p uint16) {
	binary.BigEndian.PutUint16(t[:2], p)
}

func (t TCPHeader) DestPort() uint16 {
	return binary.BigEndian.Uint16(t[2:4])
}

func (t TCPHeader) SetDestPort(p uint16) {
	binary.BigEndian.PutUint16(t[2:4], p)
}

func (t TCPHeader) SeqNum() uint32 {
	return binary.BigEndian.Uint32(t[4:8])
}

func (t TCPHeader) SetSeqNum(n uint32) {
	binary.BigEndian.PutUint32(t[4:8], n)
}

func (t TCPHeader) AckNum() uint32 {
	return binary.BigEndian.Uint32(t[8:12])
}

func (t TCPHeader) SetAckNum(n uint32) {
	binary.BigEndian.PutUint32(t[8:12], n)
}

func (t TCPHeader) DataOffset() uint8 {
	return t[12] >> 4
}

func (t TCPHeader) SetDataOffset(off uint8) {
	t[12] &= 0xf
	t[12] |= off << 4
}

func (t TCPHeader) Flag(f Flag) bool {
	if f == NS {
		return t[12]&1 == 1
	} else {
		return t[13]&(1<<(8-f)) != 0
	}
}

func (t TCPHeader) SetFlag(f Flag, b bool) {
	if f == NS {
		if b {
			t[12] |= 1
		} else {
			t[12] &= 0xfe
		}
	} else {
		if b {
			t[13] |= (1 << (8 - f))
		} else {
			t[13] &= ^(1 << (8 - f))
		}
	}
}

func (t TCPHeader) WindowSize() uint16 {
	return binary.BigEndian.Uint16(t[14:16])
}

func (t TCPHeader) SetWindowSize(size uint16) {
	binary.BigEndian.PutUint16(t[14:16], size)
}

func (t TCPHeader) Checksum() uint16 {
	return binary.BigEndian.Uint16(t[16:18])
}

func (t TCPHeader) SetChecksum(checksum uint16) {
	binary.BigEndian.PutUint16(t[16:18], checksum)
}

func (t TCPHeader) UrgPointer() uint16 {
	return binary.BigEndian.Uint16(t[18:20])
}

func (t TCPHeader) SetUrgPointer(urg uint16) {
	binary.BigEndian.PutUint16(t[18:20], urg)
}

func (t TCPHeader) TCPOptions() ([]*TCPOption, error) {
	var res []*TCPOption
	reader := bytes.NewReader(t[20:])
	for reader.Len() != 0 {
		opt, err := ReadTCPOption(reader)
		if err != nil {
			return nil, essentials.AddCtx("TCPOptions", err)
		}
		res = append(res, opt)
	}
	return res, nil
}

type TCPOption struct {
	Kind byte
	Data []byte
}

func ReadTCPOption(r *bytes.Reader) (*TCPOption, error) {
	kind, err := r.ReadByte()
	if err != nil {
		return nil, io.EOF
	}
	if kind < 2 {
		return &TCPOption{Kind: kind}, nil
	}
	size, err := r.ReadByte()
	if err != nil {
		return nil, io.ErrUnexpectedEOF
	}
	data := make([]byte, size)
	if _, err := r.Read(data); err != nil {
		return nil, io.ErrUnexpectedEOF
	}
	return &TCPOption{Kind: kind, Data: data}, nil
}

func (t *TCPOption) Encode() []byte {
	if t.Kind < 2 {
		return []byte{t.Kind}
	} else {
		return append([]byte{t.Kind, byte(len(t.Data))}, t.Data...)
	}
}

// A TCP4Packet is a TCP packet contained in an IPv4
// packet.
type TCP4Packet []byte

// Valid checks that the packet can be used.
func (t TCP4Packet) Valid() bool {
	ipPacket := IPv4Packet(t)
	if !ipPacket.Valid() {
		return false
	}
	if len(ipPacket.Payload()) < 20 {
		return false
	}
	header := TCPHeader(ipPacket.Payload())
	return int(header.DataOffset()*4) <= len(ipPacket.Payload())
}

// SourceAddr gets the source IPv4 address and port.
//
// This assumes that the packet is valid.
func (t TCP4Packet) SourceAddr() *net.TCPAddr {
	return &net.TCPAddr{
		IP:   IPv4Packet(t).SourceAddr(),
		Port: int(t.Header().SourcePort()),
	}
}

// DestAddr gets the destination IPv4 address and port.
//
// This assumes that the packet is valid.
func (t TCP4Packet) DestAddr() *net.TCPAddr {
	return &net.TCPAddr{
		IP:   IPv4Packet(t).DestAddr(),
		Port: int(t.Header().DestPort()),
	}
}

// Header gets the TCP header.
//
// This assumes that the packet is valid.
func (t TCP4Packet) Header() TCPHeader {
	h := TCPHeader(IPv4Packet(t).Payload())
	hSize := h.DataOffset() * 4
	return h[:hSize]
}

// Payload gets the TCP data payload.
//
// This assumes that the packet is valid.
func (t TCP4Packet) Payload() []byte {
	hSize := len(t.Header())
	return IPv4Packet(t).Payload()[hSize:]
}

// Checksum computes the packet's checksum.
// Zero means the checksum is correct.
//
// This assumes that the packet is valid.
func (t TCP4Packet) Checksum() uint16 {
	ipPacket := IPv4Packet(t)
	header := t.Header()
	fakePacket := bytes.NewBuffer(nil)
	fakePacket.Write(ipPacket.SourceAddr())
	fakePacket.Write(ipPacket.DestAddr())
	fakePacket.WriteByte(0)
	fakePacket.WriteByte(ProtocolNumberTCP)
	binary.Write(fakePacket, binary.BigEndian, uint16(len(header)+len(t.Payload())))
	fakePacket.Write(ipPacket.Payload())
	return InternetChecksum(fakePacket.Bytes())
}

// SetChecksum computes the correct checksum and inserts
// it into the packet.
//
// This assumes that the packet is valid.
func (t TCP4Packet) SetChecksum() {
	t.Header().SetChecksum(0)
	t.Header().SetChecksum(t.Checksum())
}

type tcpSegment struct {
	Start uint32
	Data  []byte
	Fin   bool
}
