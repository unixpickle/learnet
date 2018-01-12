package dnsproto

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
)

// A Record is a DNS resource record.
//
// There are various types that implement this interface,
// including ARecord and DomainRecord.
type Record interface {
	Name() DomainName
	Type() uint16
	Class() uint16
	TTL() uint32
	Data() []byte
}

// A GenericRecord implements the bare-minimum Record
// functionality.
type GenericRecord struct {
	NameValue  DomainName
	TypeValue  uint16
	ClassValue uint16
	TTLValue   uint32
	DataValue  []byte
}

// Name returns the record's name field.
func (g *GenericRecord) Name() DomainName {
	return g.NameValue
}

// Type returns the record's type field.
func (g *GenericRecord) Type() uint16 {
	return g.TypeValue
}

// Class returns the record's class field.
func (g *GenericRecord) Class() uint16 {
	return g.ClassValue
}

// TTL returns the record's TTL field.
func (g *GenericRecord) TTL() uint32 {
	return g.TTLValue
}

// Data returns the record's data.
func (g *GenericRecord) Data() []byte {
	return g.DataValue
}

// String generates a human-readable representation.
func (g *GenericRecord) String() string {
	return fmt.Sprintf("<record type %d: %s>", g.TypeValue, hex.EncodeToString(g.DataValue))
}

// An ARecord is an IPv4 address record.
type ARecord struct {
	*GenericRecord
}

// IP gets the IPv4 address contained in the record.
func (a *ARecord) IP() net.IP {
	return net.IP(a.Data())
}

// String generates a human-readable representation.
func (a *ARecord) String() string {
	return "<A record: " + a.IP().String() + ">"
}

// A DomainRecord is a resource record whose data is a
// domain name.
type DomainRecord struct {
	*GenericRecord

	DomainDataValue DomainName
}

// DomainName returns the domain in this record.
func (d *DomainRecord) DomainData() DomainName {
	return d.DomainDataValue
}

// An SOARecord is a Start of Authority resource record.
type SOARecord struct {
	*GenericRecord

	MasterName      DomainName
	ResponsibleName DomainName
	Serial          uint32
	Refresh         uint32
	Retry           uint32
	Expire          uint32
	Minimum         uint32
}

// String returns a summary of the record.
func (s *SOARecord) String() string {
	return fmt.Sprintf("<SOA record: %s %s %d %d %d %d %d>",
		s.MasterName, s.ResponsibleName, s.Serial, s.Refresh, s.Retry, s.Expire, s.Minimum)
}

func decodeRecord(m *messageReader) (Record, error) {
	g := &GenericRecord{}
	var length uint16
	err := m.ReadFields(&g.NameValue, &g.TypeValue, &g.ClassValue, &g.TTLValue, &length)
	if err != nil {
		return nil, err
	}
	g.DataValue, err = m.ReadN(int(length))
	if err != nil {
		return nil, err
	}

	switch g.Type() {
	case RecordTypeA:
		if len(g.DataValue) != 4 {
			return nil, errors.New("invalid size for A record")
		}
		return &ARecord{GenericRecord: g}, nil
	case RecordTypeNS, RecordTypeCNAME, RecordTypePTR:
		oldOff := m.Offset()
		m.Backtrack(len(g.DataValue))
		res := &DomainRecord{GenericRecord: g}
		err := m.ReadFields(&res.DomainDataValue)
		if err != nil || m.Offset() != oldOff {
			return nil, errors.New("invalid domain record")
		}
		return res, nil
	case RecordTypeSOA:
		oldOff := m.Offset()
		m.Backtrack(len(g.DataValue))
		res := &SOARecord{GenericRecord: g}
		err := m.ReadFields(&res.MasterName, &res.ResponsibleName, &res.Serial, &res.Refresh,
			&res.Retry, &res.Expire, &res.Minimum)
		if err != nil || m.Offset() != oldOff {
			return nil, errors.New("invalid SOA record")
		}
		return res, nil
	}

	return g, nil
}

func encodeRecord(m *messageWriter, record Record) error {
	err := m.WriteFields(record.Name(), record.Type(), record.Class(), record.TTL())
	if err != nil {
		return err
	}
	switch record := record.(type) {
	case *DomainRecord:
		return m.WriteWithLength(record.DomainData())
	case *SOARecord:
		return m.WriteWithLength(record.MasterName, record.ResponsibleName, record.Serial,
			record.Refresh, record.Retry, record.Expire, record.Minimum)
	default:
		return m.WriteFields(uint16(len(record.Data())), record.Data())
	}
}
