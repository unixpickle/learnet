package dnsproto

import (
	"errors"

	"github.com/unixpickle/essentials"
)

// A Message is a DNS message.
type Message struct {
	Header      *Header
	Questions   []*Question
	Answers     []Record
	Authorities []Record
	Additional  []Record
}

// QueryMessage creates a Message for a typical DNS query.
func QueryMessage(domain DomainName, recordType uint16, recursive bool) *Message {
	return &Message{
		Header: &Header{
			Opcode:           OpcodeQuery,
			RecursionDesired: recursive,
			QuestionCount:    1,
		},
		Questions: []*Question{{Domain: domain, Type: recordType, Class: RecordClassIN}},
	}
}

// DecodeMessage reads a Message from binary data.
func DecodeMessage(data []byte) (msg *Message, err error) {
	defer essentials.AddCtxTo("decode DNS message", &err)

	m := newMessageReader(data)

	msg = &Message{}
	if msg.Header, err = decodeHeader(m); err != nil {
		return nil, err
	}

	for i := 0; i < int(msg.Header.QuestionCount); i++ {
		if question, err := decodeQuestion(m); err != nil {
			if err == ErrBufferUnderflow && msg.Header.Truncated {
				return msg, nil
			}
			return nil, err
		} else {
			msg.Questions = append(msg.Questions, question)
		}
	}

	counts := []uint16{msg.Header.AnswerCount, msg.Header.AuthorityCount,
		msg.Header.AdditionalCount}
	fields := []*[]Record{&msg.Answers, &msg.Authorities, &msg.Additional}
	for i, field := range fields {
		for j := 0; j < int(counts[i]); j++ {
			if record, err := decodeRecord(m); err != nil {
				if err == ErrBufferUnderflow && msg.Header.Truncated {
					return msg, nil
				}
				return nil, err
			} else {
				*field = append(*field, record)
			}
		}
	}

	if m.Remaining() != 0 {
		return nil, errors.New("excess data")
	}

	return msg, nil
}

// Encode encodes the message as binary data.
func (m *Message) Encode() (data []byte, err error) {
	defer essentials.AddCtxTo("encode DNS message", &err)

	writer := &messageWriter{}
	if err := m.Header.encode(writer); err != nil {
		return nil, err
	}
	for _, question := range m.Questions {
		if err := question.encode(writer); err != nil {
			return nil, err
		}
	}
	for _, records := range [][]Record{m.Answers, m.Authorities, m.Additional} {
		for _, record := range records {
			if err := encodeRecord(writer, record); err != nil {
				return nil, err
			}
		}
	}
	return writer.Bytes(), nil
}

// AutoFill fills in the header with information that can
// be derived from the other fields of the packet.
// For example, it sets the AnswerCount field.
func (m *Message) AutoFill() {
	m.Header.QuestionCount = uint16(len(m.Questions))
	m.Header.AnswerCount = uint16(len(m.Answers))
	m.Header.AuthorityCount = uint16(len(m.Authorities))
	m.Header.AdditionalCount = uint16(len(m.Additional))
}

// Records gets all the records in order.
func (m *Message) Records() []Record {
	return append(append(append([]Record{}, m.Answers...), m.Authorities...), m.Additional...)
}
