package dnsproto

// HeaderSize is the size of the DNS header.
const HeaderSize = 12

// Header represents a 12 byte DNS header.
type Header []byte

// Identifier gets the identifier field.
func (h Header) Identifier() uint16 {
	return getShort(h)
}

// SetIdentifier sets the identifier field.
func (h Header) SetIdentifier(id uint16) {
	setShort(h, id)
}

// ResponseFlag gets the response flag.
func (h Header) ResponseFlag() bool {
	return getBit(h[2], 0)
}

// SetResponseFlag sets the response flag.
func (h Header) SetResponseFlag(flag bool) {
	h[2] = setBit(h[2], 0, flag)
}

// Opcode gets the opcode field.
func (h Header) Opcode() int {
	return int((h[2] >> 3) & 0xf)
}

// SetOpcode sets the opcode field.
func (h Header) SetOpcode(opcode int) {
	h[2] &= 0x8f
	h[2] |= byte((opcode & 0xf) << 3)
}

// Authoritative gets the authoritative flag.
func (h Header) Authoritative() bool {
	return getBit(h[2], 5)
}

// SetAuthoritative sets the authoritative flag.
func (h Header) SetAuthoritative(flag bool) {
	h[2] = setBit(h[2], 5, flag)
}

// Truncation gets the truncation flag.
func (h Header) Truncation() bool {
	return getBit(h[2], 6)
}

// SetTruncation sets the truncation flag.
func (h Header) SetTruncation(flag bool) {
	h[2] = setBit(h[2], 6, flag)
}

// RecursionDesired gets the recursion desired flag.
func (h Header) RecursionDesired() bool {
	return getBit(h[2], 7)
}

// SetRecursionDesired sets the recursion desired flag.
func (h Header) SetRecursionDesired(flag bool) {
	h[2] = setBit(h[2], 7, flag)
}

// RecursionAvailable gets the recursion available flag.
func (h Header) RecursionAvailable() bool {
	return getBit(h[3], 0)
}

// SetRecursionAvailable sets the recursion available
// flag.
func (h Header) SetRecursionAvailable(flag bool) {
	h[3] = setBit(h[3], 0, flag)
}

// ResponseCode gets the response code field.
func (h Header) ResponseCode() int {
	return int(h[3] & 0xf)
}

// SetResponseCode sets the response code field.
func (h Header) SetResponseCode(code int) {
	h[3] &= 0xf0
	h[3] |= byte(code)
}

// QuestionCount gets the question count field.
func (h Header) QuestionCount() uint16 {
	return getShort(h[4:])
}

// SetQuestionCount sets the question count field.
func (h Header) SetQuestionCount(count uint16) {
	setShort(h[4:], count)
}

// AnswerCount gets the answer count field.
func (h Header) AnswerCount() uint16 {
	return getShort(h[6:])
}

// SetAnswerCount gets the answer count field.
func (h Header) SetAnswerCount(count uint16) {
	setShort(h[6:], count)
}

// AuthorityCount gets the authority count field.
func (h Header) AuthorityCount() uint16 {
	return getShort(h[8:])
}

// SetAuthorityCount sets the authority count field.
func (h Header) SetAuthorityCount(count uint16) {
	setShort(h[8:], count)
}

// AdditionalCount gets the additional count field.
func (h Header) AdditionalCount() uint16 {
	return getShort(h[10:])
}

// SetAdditionalCount sets the additional count field.
func (h Header) SetAdditionalCount(count uint16) {
	setShort(h[10:], count)
}
