package dnsproto

type Opcode uint8

const (
	OpcodeQuery  Opcode = 0
	OpcodeIQuery        = 1
	OpcodeStatus        = 2
	OpcodeNotify        = 4
	OpcodeUpdate        = 5
)

type ResponseCode uint8

const (
	ResponseCodeNoError ResponseCode = iota
	ResponseCodeFormatError
	ResponseCodeServerFailure
	ResponseCodeNXDomain
	ResponseCodeNotImplemented
	ResponseCodeRefused
	ResponseCodeYXDomain
	ResponseCodeYXRRSet
	ResponseCodeNXRRSet
	ResponseCodeNotAuth
	ResponseCodeNotZone
)

const (
	RecordTypeA     = 1
	RecordTypeNS    = 2
	RecordTypeCNAME = 5
	RecordTypeSOA   = 6
	RecordTypePTR   = 12
	RecordTypeMX    = 15
	RecordTypeTXT   = 16
	RecordTypeAAAA  = 28
)

const (
	RecordClassIN = 1
)
