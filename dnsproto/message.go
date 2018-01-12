package dnsproto

import (
	"errors"

	"github.com/unixpickle/essentials"
)

// A Message is a DNS message.
type Message struct {
	Header    *Header
	Questions []*Question
	Records   []Record
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

	recordCount := int(msg.Header.AnswerCount + msg.Header.AuthorityCount +
		msg.Header.AdditionalCount)
	for i := 0; i < recordCount; i++ {
		if record, err := decodeRecord(m); err != nil {
			if err == ErrBufferUnderflow && msg.Header.Truncated {
				return msg, nil
			}
			return nil, err
		} else {
			msg.Records = append(msg.Records, record)
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
	for _, record := range m.Records {
		if err := encodeRecord(writer, record); err != nil {
			return nil, err
		}
	}

	return writer.Bytes(), nil
}
