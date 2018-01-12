package dnsproto

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/unixpickle/essentials"
)

const ClassNumberIN = 1

var OutOfBoundsErr = errors.New("index out of bounds")

// A Question represents the question section of a DNS
// message.
type Question struct {
	Labels []string
	Type   int
	Class  int
}

// Encode writes the Question into a DNS message buffer.
func (q *Question) Encode(out *bytes.Buffer) (err error) {
	defer essentials.AddCtxTo("encode DNS question", &err)
	if err := encodeLabels(out, q.Labels); err != nil {
		return err
	}
	binary.Write(out, binary.BigEndian, uint16(q.Type))
	binary.Write(out, binary.BigEndian, uint16(q.Class))
	return nil
}

// A Message is a DNS message.
type Message struct {
	Header   Header
	Question *Question
	Records  []Record
}

// DecodeMessage reads a Message from binary data.
func DecodeMessage(data []byte) (msg *Message, err error) {
	defer essentials.AddCtxTo("decode DNS message", &err)

	if len(data) < 12 {
		return nil, errors.New("message is too small")
	}

	labels, endIdx, err := readLabels(data, 12, len(data))
	if err != nil {
		return nil, err
	}
	if endIdx+4 > len(data) {
		return nil, OutOfBoundsErr
	}

	msg = &Message{
		Header: Header(data[:12]),
		Question: &Question{
			Labels: labels,
			Type:   int(getShort(data[endIdx:])),
			Class:  int(getShort(data[endIdx+2:])),
		},
	}

	msg.Records, err = readResourceRecords(data, endIdx+4)

	return
}

// Encode encodes the message as binary data.
func (m Message) Encode() (data []byte, err error) {
	defer essentials.AddCtxTo("encode DNS message", &err)
	var buf bytes.Buffer
	buf.Write(m.Header)
	if err := m.Question.Encode(&buf); err != nil {
		return nil, err
	}
	for _, rr := range m.Records {
		if err := rr.Encode(&buf); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func readResourceRecords(data []byte, curIdx int) (records []Record, err error) {
	var labels []string
	for curIdx < len(data) {
		labels, curIdx, err = readLabels(data, curIdx, len(data))
		if err != nil {
			return
		} else if curIdx+10 > len(data) {
			return records, OutOfBoundsErr
		}
		dataLen := int(getShort(data[curIdx+8:]))
		if curIdx+10+dataLen > len(data) {
			return records, OutOfBoundsErr
		}
		generic := &GenericRecord{
			NameValue:  labels,
			TypeValue:  int(getShort(data[curIdx:])),
			ClassValue: int(getShort(data[curIdx+2:])),
			TTLValue:   getInt(data[curIdx+4:]),
			DataValue:  data[curIdx+10 : curIdx+10+dataLen],
		}
		var specific Record
		specific, err = readSpecificRecord(generic, curIdx+10, data)
		if err != nil {
			return
		}
		records = append(records, specific)
		curIdx += 10 + dataLen
	}
	return
}

// readLabels reads a list of labels at the given offset
// in the message.
//
// The returned endIdx is the index of the byte after the
// last byte making up the labels.
// This can be used to read fields that proceed a domain
// name field.
//
// This may fail if the labels lead to out-of-bounds
// accesses, loops, etc.
func readLabels(msg []byte, offset, limit int) ([]string, int, error) {
	if offset < 0 || offset >= limit {
		return nil, 0, OutOfBoundsErr
	}
	labelSize := int(msg[offset])
	if labelSize&0xc0 == 0xc0 {
		if offset+1 >= limit {
			return nil, 0, OutOfBoundsErr
		}
		ptr := (int(labelSize&0x3f) << 8) | int(msg[offset+1])
		labels, _, err := readLabels(msg, ptr, offset)
		return labels, offset + 2, err
	} else if labelSize&0xc0 != 0 {
		return nil, 0, errors.New("unrecognized field value")
	} else if offset+labelSize >= limit {
		return nil, 0, OutOfBoundsErr
	} else if labelSize == 0 {
		return nil, offset + 1, nil
	}
	label := string(msg[offset+1 : offset+labelSize+1])
	nextLabels, endIdx, err := readLabels(msg, offset+labelSize+1, limit)
	if err != nil {
		return nil, 0, err
	}
	return append([]string{label}, nextLabels...), endIdx, nil
}

func encodeLabels(out *bytes.Buffer, labels []string) error {
	// TODO: perform compression here by searching earlier in
	// the output buffer for the exact contents of the label.
	for _, label := range labels {
		if len(label) > 63 {
			return errors.New("label is too long to encode")
		}
		labelBytes := []byte(label)
		out.WriteByte(byte(len(labelBytes)))
		out.Write(labelBytes)
	}
	out.WriteByte(0)
	return nil
}
