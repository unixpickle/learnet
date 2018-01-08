package ipstack

// InternetChecksum computes the checksum of the data.
//
// If the data is uncorrupted, the checksum should be 0.
//
// The resulting 16-bit integer should be encoded as big
// endian when sent over the wire.
func InternetChecksum(data []byte) uint16 {
	// Adapted from C example in RFC 1071:
	// https://tools.ietf.org/html/rfc1071.

	var sum uint32

	for len(data) >= 2 {
		sum += (uint32(data[0]) << 8) | uint32(data[1])
		data = data[2:]
	}

	if len(data) == 1 {
		sum += uint32(data[0]) << 8
	}

	for (sum >> 16) != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}

	return ^uint16(sum)
}
