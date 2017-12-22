// +build darwin

package tunnet

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	pfSystem         = 32
	sysprotoControl  = 2
	afSysControl     = 2
	ioctlCTLIOCGINFO = 0xc0644e03
	utunOptIfname    = 2

	utunControl = "com.apple.net.utun_control"
)

type utunSocket struct {
	fd   int
	name string
}

func openUtunSocket() (res *utunSocket, err error) {
	fd, err := unix.Socket(pfSystem, unix.SOCK_DGRAM, sysprotoControl)
	if err != nil {
		return nil, err
	}
	socket := &utunSocket{fd: fd}

	defer func() {
		if err != nil {
			unix.Close(socket.fd)
		}
	}()

	// struct ctl_info
	ctlInfo := make([]byte, 128)
	copy(ctlInfo[4:], []byte(utunControl))
	if ok, err := socket.ioctlWithData(ioctlCTLIOCGINFO, ctlInfo); !ok {
		return nil, err
	}

	// struct sockaddr_ctl
	addrData := make([]byte, 32)
	addrData[0] = 32
	addrData[1] = unix.AF_SYSTEM
	addrData[2] = afSysControl
	copy(addrData[4:], ctlInfo[:4])
	_, _, sysErr := unix.Syscall(unix.SYS_CONNECT, uintptr(socket.fd),
		uintptr(unsafe.Pointer(&addrData[0])), uintptr(32))
	if sysErr != 0 {
		return nil, sysErr
	}

	nameData := make([]byte, 32)
	nameLen := uintptr(len(nameData))
	_, _, sysErr = unix.Syscall6(unix.SYS_GETSOCKOPT, uintptr(socket.fd), sysprotoControl,
		utunOptIfname, uintptr(unsafe.Pointer(&nameData[0])), uintptr(unsafe.Pointer(&nameLen)), 0)
	if sysErr != 0 {
		return nil, sysErr
	}

	socket.name = string(nameData[:nameLen-1])

	return socket, nil
}

func (u *utunSocket) Read(buffer []byte) (int, error) {
	for {
		amount, err := unix.Read(u.fd, buffer)
		if err == nil {
			return amount, nil
		} else if err == unix.EINTR {
			// TODO: can amount be > 0?
			continue
		} else {
			return amount, err
		}
	}
}

func (u *utunSocket) Write(buffer []byte) (int, error) {
	return unix.Write(u.fd, buffer)
}

func (u *utunSocket) Shutdown() error {
	return unix.Shutdown(u.fd, unix.SHUT_RDWR)
}

func (u *utunSocket) ioctlWithData(command int, data []byte) (ok bool, err syscall.Errno) {
	if data != nil {
		_, _, err = unix.Syscall(unix.SYS_IOCTL, uintptr(u.fd), uintptr(command),
			uintptr(unsafe.Pointer(&data[0])))
	} else {
		_, _, err = unix.Syscall(unix.SYS_IOCTL, uintptr(u.fd), uintptr(command), uintptr(0))
	}
	if err != 0 {
		return
	} else {
		return true, 0
	}
}
