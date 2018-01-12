package dnsproto

func getBit(b byte, idx uint) bool {
	return (b & (1 << (7 - idx))) != 0
}

func setBit(b byte, idx uint, flag bool) byte {
	if flag {
		return b | (1 << (7 - idx))
	} else {
		return b & (0xff ^ (1 << (7 - idx)))
	}
}

func getShort(b []byte) uint16 {
	return (uint16(b[0]) << 8) | uint16(b[1])
}

func setShort(b []byte, val uint16) {
	b[0] = byte(val >> 8)
	b[1] = byte(val)
}

func getInt(b []byte) uint32 {
	return (uint32(b[0]) << 24) | (uint32(b[1]) << 16) | (uint32(b[2]) << 8) | uint32(b[3])
}

func setInt(b []byte, val uint32) {
	b[0] = byte(val >> 24)
	b[1] = byte(val >> 16)
	b[2] = byte(val >> 8)
	b[3] = byte(val)
}
