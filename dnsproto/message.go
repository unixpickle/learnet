package dnsproto

import (
	"errors"

	"github.com/unixpickle/essentials"
)

var OutOfBoundsErr = errors.New("index out of bounds")

// A Question represents the question section of a DNS
// message.
type Question struct {
	Labels []string
	Type   int
	Class  int
}

// A Message is a DNS message.
type Message struct {
	Header   Header
	Question *Question
	Records  []interface{} // TODO: interface for RRs
}

// DecodeMessage reads a Message from binary data.
func DecodeMessage(data []byte) (msg *Message, err error) {
	defer essentials.AddCtxTo("decode message", &err)

	labels, endIdx, err := readLabels(data, 12, len(data))
	if err != nil {
		return nil, err
	}
	if endIdx+4 > len(data) {
		return nil, err
	}
	msg = &Message{
		Question: &Question{
			Labels: labels,
			Type:   int(getShort(data[endIdx:])),
			Class:  int(getShort(data[endIdx+2:])),
		},
	}

	// TODO: read records here.

	return msg, nil
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
