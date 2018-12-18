// +build linux

package tunnet

import (
	"bytes"
	"encoding/binary"
	"net"
	"syscall"
)

var systemByteOrder = binary.LittleEndian

// packSockaddr4 packs a sockaddr_in struct.
func packSockaddr4(addr net.IP, port int) []byte {
	ip4 := addr.To4()
	if ip4 == nil {
		panic("must take an IPv4 address")
	}
	var buf bytes.Buffer
	binary.Write(&buf, systemByteOrder, uint16(syscall.AF_INET))
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
