package dnsproto

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

var (
	ErrBufferUnderflow    = errors.New("buffer underflow")
	ErrInvalidNamePointer = errors.New("invalid name pointer")
)

// An encoder is an object which can be encoded into a DNS
// message.
type encoder interface {
	encode(m *messageWriter) error
}

// A messageWriter composes an outgoing DNS message.
type messageWriter struct {
	buf bytes.Buffer
}

// WriteFields writes a sequence of field values (either
// integers, DomainNames, or byte slices) to the buffer.
func (m *messageWriter) WriteFields(fields ...interface{}) error {
	for _, field := range fields {
		switch field := field.(type) {
		case DomainName:
			if err := m.writeDomain(field); err != nil {
				return err
			}
		case []byte:
			m.buf.Write(field)
		default:
			binary.Write(&m.buf, binary.BigEndian, field)
		}
	}
	return nil
}

// WriteLengthAndDomain writes the 16-bit length of an
// encoded domain followed by the domain itself.
func (m *messageWriter) WriteLengthAndDomain(domain DomainName) error {
	m.buf.WriteByte(0)
	m.buf.WriteByte(0)
	start := m.buf.Len()
	if err := m.WriteFields(domain); err != nil {
		return err
	}
	length := m.buf.Len() - start
	raw := m.buf.Bytes()
	raw[start-2] = byte(length >> 8)
	raw[start-1] = byte(length)
	return nil
}

// Bytes returns the bytes written so far.
func (m *messageWriter) Bytes() []byte {
	return m.buf.Bytes()
}

func (m *messageWriter) writeDomain(d DomainName) error {
	if err := d.Validate(); err != nil {
		return err
	}
	for _, label := range d {
		if len(label) > 63 {
			return errors.New("label is too long to encode")
		}
		labelBytes := []byte(label)
		m.buf.WriteByte(byte(len(labelBytes)))
		m.buf.Write(labelBytes)
	}
	m.buf.WriteByte(0)
	return nil
}

// A messageReader decodes an incoming DNS message.
type messageReader struct {
	data   []byte
	reader *bytes.Reader
}

func newMessageReader(data []byte) *messageReader {
	return &messageReader{data: data, reader: bytes.NewReader(data)}
}

// ReadFields reads a sequence of field values (either
// integers or DomainNames) into pointers.
func (m *messageReader) ReadFields(fields ...interface{}) error {
	for _, field := range fields {
		switch field := field.(type) {
		case *DomainName:
			var err error
			*field, err = m.readDomain()
			if err != nil {
				return err
			}
		default:
			if err := binary.Read(m.reader, binary.BigEndian, field); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *messageReader) ReadByte() (byte, error) {
	if b, err := m.reader.ReadByte(); err != nil {
		return 0, ErrBufferUnderflow
	} else {
		return b, nil
	}
}

func (m *messageReader) ReadN(n int) ([]byte, error) {
	res := make([]byte, n)
	if got, err := m.reader.Read(res); err != nil || got != n {
		return nil, ErrBufferUnderflow
	}
	return res, nil
}

func (m *messageReader) Offset() int {
	return len(m.data) - m.reader.Len()
}

func (m *messageReader) Backtrack(n int) {
	m.reader.Seek(-int64(n), io.SeekCurrent)
}

func (m *messageReader) Remaining() int {
	return m.reader.Len()
}

func (m *messageReader) readDomain() (DomainName, error) {
	var result DomainName
	for {
		length, err := m.ReadByte()
		if err != nil {
			return nil, err
		}
		if length&0xc0 == 0xc0 {
			length2, err := m.ReadByte()
			if err != nil {
				return nil, err
			}
			ptr := (int(length&0x3f) << 8) | int(length2)
			if ptr >= m.Offset()-2 {
				return nil, ErrInvalidNamePointer
			}
			preReader := newMessageReader(m.data[:m.Offset()-2])
			preReader.reader.Seek(int64(ptr), io.SeekStart)
			ptrDomain, err := preReader.readDomain()
			if err != nil {
				return nil, ErrInvalidNamePointer
			}
			result = append(result, ptrDomain...)
			return result, result.Validate()
		} else if length&0xc0 != 0 {
			return nil, errors.New("invalid label length field")
		} else if length == 0 {
			return result, result.Validate()
		}
		labelData, err := m.ReadN(int(length))
		if err != nil {
			return nil, err
		}
		result = append(result, string(labelData))
	}
}

func getBit(b byte, idx uint) bool {
	return (b & (1 << (7 - idx))) != 0
}

func setBit(b byte, idx uint, flag bool) byte {
	if flag {
		return b | (1 << (7 - idx))
	} else {
		return b & (0xff ^ (1 << (7 - idx)))
	}
}
