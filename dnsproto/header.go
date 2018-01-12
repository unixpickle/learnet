package dnsproto

// A Header stores the header of a DNS message.
type Header struct {
	Identifier uint16
	IsResponse bool
	Opcode     Opcode

	Authoritative      bool
	Truncated          bool
	RecursionDesired   bool
	RecursionAvailable bool

	ResponseCode ResponseCode

	QuestionCount   uint16
	AnswerCount     uint16
	AuthorityCount  uint16
	AdditionalCount uint16
}

func decodeHeader(m *messageReader) (*Header, error) {
	var flags uint16
	h := &Header{}
	err := m.ReadFields(&h.Identifier, &flags, &h.QuestionCount, &h.AnswerCount,
		&h.AuthorityCount, &h.AdditionalCount)
	if err != nil {
		return nil, err
	}
	if flags&(1<<15) != 0 {
		h.IsResponse = true
	}
	h.Opcode = Opcode(flags>>11) & 0xf
	if flags&(1<<10) != 0 {
		h.Authoritative = true
	}
	if flags&(1<<9) != 0 {
		h.Truncated = true
	}
	if flags&(1<<8) != 0 {
		h.RecursionDesired = true
	}
	if flags&(1<<7) != 0 {
		h.RecursionAvailable = true
	}
	h.ResponseCode = ResponseCode(flags) & 0xf
	return h, nil
}

func (h *Header) encode(m *messageWriter) error {
	var flags uint16
	if h.IsResponse {
		flags |= 1 << 15
	}
	flags |= uint16(h.Opcode&0xf) << 11
	if h.Authoritative {
		flags |= 1 << 10
	}
	if h.Truncated {
		flags |= 1 << 9
	}
	if h.RecursionDesired {
		flags |= 1 << 8
	}
	if h.RecursionAvailable {
		flags |= 1 << 7
	}
	flags |= uint16(h.ResponseCode & 0xf)
	return m.WriteFields(h.Identifier, flags, h.QuestionCount, h.AnswerCount,
		h.AuthorityCount, h.AdditionalCount)
}
