package dnsproto

import (
	"errors"
	"net"
)

// TODO: add AAAA record!
const (
	RecordTypeA     = 1
	RecordTypeNS    = 2
	RecordTypeCNAME = 5
	RecordTypeSOA   = 6
	RecordTypePTR   = 12
	RecordTypeMX    = 15
	RecordTypeTXT   = 16
)

// A Record is a DNS resource record.
type Record interface {
	Name() []string
	Type() int
	Class() int
	TTL() uint32
	Data() []byte
}

func readSpecificRecord(g *GenericRecord, dataOffset int, packet []byte) (Record, error) {
	switch g.Type() {
	case RecordTypeNS, RecordTypeCNAME, RecordTypePTR:
		return readDomainRecord(g, dataOffset, packet)
	case RecordTypeA:
		if len(g.Data()) != 4 {
			return nil, errors.New("unexpected data size for A record")
		}
		return ARecord{GenericRecord: g}, nil
	}
	// TODO: support SOA records.
	return g, nil
}

// A GenericRecord implements the bare-minimum Record
// functionality.
type GenericRecord struct {
	NameValue  []string
	TypeValue  int
	ClassValue int
	TTLValue   uint32
	DataValue  []byte
}

// Name returns the record's name field.
func (g *GenericRecord) Name() []string {
	return g.NameValue
}

// Type returns the record's type field.
func (g *GenericRecord) Type() int {
	return g.TypeValue
}

// Class returns the record's class field.
func (g *GenericRecord) Class() int {
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

// An ARecord is an IPv4 address record.
type ARecord struct {
	*GenericRecord
}

// IP gets the IPv4 address contained in the record.
func (a *ARecord) IP() net.IP {
	return net.IP(a.Data())
}

// A DomainRecord is a resource record containing a domain
// name.
type DomainRecord struct {
	*GenericRecord

	DomainValue []string
}

func readDomainRecord(g *GenericRecord, dataOffset int, packet []byte) (*DomainRecord, error) {
	labels, endIdx, err := readLabels(packet, dataOffset, dataOffset+len(g.Data()))
	if err != nil {
		return nil, err
	} else if endIdx != dataOffset+len(g.Data()) {
		return nil, errors.New("domain field contains extra bytes")
	}
	return &DomainRecord{GenericRecord: g, DomainValue: labels}, nil
}

// Domain returns the domain contained in this record.
func (d *DomainRecord) Domain() []string {
	return d.DomainValue
}
