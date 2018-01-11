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
