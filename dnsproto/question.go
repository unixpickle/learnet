package dnsproto

// A Question stores a query in a DNS message.
type Question struct {
	Domain DomainName
	Type   uint16
	Class  uint16
}

func decodeQuestion(m *messageReader) (*Question, error) {
	q := &Question{}
	return q, m.ReadFields(&q.Domain, &q.Type, &q.Class)
}

func (q *Question) encode(m *messageWriter) error {
	return m.WriteFields(q.Domain, q.Type, q.Class)
}
