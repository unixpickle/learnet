package tcproto

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/unixpickle/essentials"
)

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

type Header []byte

func (h Header) SourcePort() uint16 {
	return binary.BigEndian.Uint16(h[:2])
}

func (h Header) SetSourcePort(p uint16) {
	binary.BigEndian.PutUint16(h[:2], p)
}

func (h Header) DestPort() uint16 {
	return binary.BigEndian.Uint16(h[2:4])
}

func (h Header) SetDestPort(p uint16) {
	binary.BigEndian.PutUint16(h[2:4], p)
}

func (h Header) SeqNum() uint32 {
	return binary.BigEndian.Uint32(h[4:8])
}

func (h Header) SetSeqNum(n uint32) {
	binary.BigEndian.PutUint32(h[4:8], n)
}

func (h Header) AckNum() uint32 {
	return binary.BigEndian.Uint32(h[8:12])
}

func (h Header) SetAckNum(n uint32) {
	binary.BigEndian.PutUint32(h[8:12], n)
}

func (h Header) DataOffset() uint8 {
	return h[12] >> 4
}

func (h Header) SetDataOffset(off uint8) {
	h[12] &= 0xf
	h[12] |= off << 4
}

func (h Header) Flag(f Flag) bool {
	if f == NS {
		return h[12]&1 == 1
	} else {
		return h[13]&(1<<(8-f)) != 0
	}
}

func (h Header) SetFlag(f Flag, b bool) {
	if f == NS {
		if b {
			h[12] |= 1
		} else {
			h[12] &= 0xfe
		}
	} else {
		if b {
			h[13] |= (1 << (8 - f))
		} else {
			h[13] &= ^(1 << (8 - f))
		}
	}
}

func (h Header) WindowSize() uint16 {
	return binary.BigEndian.Uint16(h[14:16])
}

func (h Header) SetWindowSize(size uint16) {
	binary.BigEndian.PutUint16(h[14:16], size)
}

func (h Header) Checksum() uint16 {
	return binary.BigEndian.Uint16(h[16:18])
}

func (h Header) SetChecksum(checksum uint16) {
	binary.BigEndian.PutUint16(h[16:18], checksum)
}

func (h Header) UrgPointer() uint16 {
	return binary.BigEndian.Uint16(h[18:20])
}

func (h Header) SetUrgPointer(urg uint16) {
	binary.BigEndian.PutUint16(h[18:20], urg)
}

func (h Header) Options() ([]*Option, error) {
	var res []*Option
	reader := bytes.NewReader(h[20:])
	for reader.Len() != 0 {
		opt, err := ReadOption(reader)
		if err != nil {
			return nil, essentials.AddCtx("Options", err)
		}
		res = append(res, opt)
	}
	return res, nil
}

type Option struct {
	Kind byte
	Data []byte
}

func ReadOption(r *bytes.Reader) (*Option, error) {
	kind, err := r.ReadByte()
	if err != nil {
		return nil, io.EOF
	}
	if kind < 2 {
		return &Option{Kind: kind}, nil
	}
	size, err := r.ReadByte()
	if err != nil {
		return nil, io.ErrUnexpectedEOF
	}
	data := make([]byte, size)
	if _, err := r.Read(data); err != nil {
		return nil, io.ErrUnexpectedEOF
	}
	return &Option{Kind: kind, Data: data}, nil
}

func (o *Option) Encode() []byte {
	if o.Kind < 2 {
		return []byte{o.Kind}
	} else {
		return append([]byte{o.Kind, byte(len(o.Data))}, o.Data...)
	}
}
