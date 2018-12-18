// +build linux

package tunnet

import (
	"errors"
	"net"
	"syscall"
	"unsafe"

	"github.com/unixpickle/essentials"
)

// AddRoute adds an entry to the system routing table.
func AddRoute(destination, gateway net.IP, mask net.IPMask) (err error) {
	defer essentials.AddCtxTo("add route", &err)
	if destination.To4() == nil || gateway.To4() == nil || len(mask) != 4 {
		return errors.New("only IPv4 is supported")
	}
	sock, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_IP)
	if err != nil {
		return err
	}
	defer syscall.Close(sock)

	requestData := make([]byte, 120)
	copy(requestData[8:], packSockaddr4(destination, 0))
	copy(requestData[24:], packSockaddr4(gateway, 0))
	copy(requestData[40:], packSockaddr4(net.IP(mask), 0))

	// RTF_UP | RTF_GATEWAY
	systemByteOrder.PutUint16(requestData[56:58], 3)

	_, _, sysErr := syscall.Syscall(syscall.SYS_IOCTL, uintptr(sock), syscall.SIOCADDRT,
		uintptr(unsafe.Pointer(&requestData[0])))
	if sysErr != 0 {
		return sysErr
	}
	return nil
}
