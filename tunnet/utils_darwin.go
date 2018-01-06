// +build darwin

package tunnet

import (
	"bytes"
	"encoding/binary"
	"net"

	"golang.org/x/sys/unix"
)

const (
	pfRoute             = 17
	pfSystem            = 32
	sysprotoControl     = 2
	afSysControl        = 2
	ioctlCTLIOCGINFO    = 0xc0644e03
	ioctlSIOCGIFMTU     = 0xc0206933
	ioctlSIOCSIFMTU     = 0x80206934
	ioctlSIOCGIFADDR    = 0xc0206921
	ioctlSIOCGIFDSTADDR = 0xc0206922
	ioctlSIOCGIFNETMASK = 0xc0206925
	ioctlSIOCDIFADDR    = 0x80206919
	ioctlSIOCAIFADDR    = 0x8040691a
)

var systemByteOrder = binary.LittleEndian

// packSockaddr4 packs a sockaddr_in struct.
func packSockaddr4(addr net.IP, port int) []byte {
	ip4 := addr.To4()
	if ip4 == nil {
		panic("must take an IPv4 address")
	}
	var buf bytes.Buffer
	buf.WriteByte(16)
	buf.WriteByte(unix.AF_INET)
	binary.Write(&buf, binary.BigEndian, uint16(port))
	buf.Write(ip4)
	buf.Write(make([]byte, 8))
	return buf.Bytes()
}

// unpackSockaddr4 unpacks a sockaddr_in struct.
func unpackSockaddr4(data []byte) (net.IP, int) {
	if len(data) != 16 {
		panic("unexpected struct length")
	}
	var port uint16
	binary.Read(bytes.NewReader(data[2:4]), binary.BigEndian, &port)
	return data[4:8], int(port)
}
