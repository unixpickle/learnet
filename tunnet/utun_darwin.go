// +build darwin

package tunnet

import (
	"bytes"
	"errors"
	"net"
	"syscall"
	"unsafe"

	"github.com/unixpickle/essentials"

	"golang.org/x/sys/unix"
)

const (
	utunOptIfname = 2
	utunControl   = "com.apple.net.utun_control"
)

// MakeTunnel creates a new tunnel interface.
func MakeTunnel() (Tunnel, error) {
	tun, err := openUtunSocket()
	err = essentials.AddCtx("make tunnel", err)
	return &posixTunnel{tun}, err
}

type utunSocket struct {
	fdTunnelBase
}

func openUtunSocket() (res *utunSocket, err error) {
	fd, err := unix.Socket(pfSystem, unix.SOCK_DGRAM, sysprotoControl)
	if err != nil {
		return nil, err
	}
	socket := &utunSocket{fdTunnelBase{fd: fd}}

	defer func() {
		if err != nil {
			unix.Close(socket.fd)
		}
	}()

	controlID, err := socket.getControlID()
	if err != nil {
		return nil, err
	}

	if err := socket.connectToControl(controlID); err != nil {
		return nil, err
	}

	nameData := make([]byte, 32)
	nameLen := uintptr(len(nameData))
	_, _, sysErr := unix.Syscall6(unix.SYS_GETSOCKOPT, uintptr(socket.fd), sysprotoControl,
		utunOptIfname, uintptr(unsafe.Pointer(&nameData[0])), uintptr(unsafe.Pointer(&nameLen)), 0)
	if sysErr != 0 {
		return nil, sysErr
	}

	socket.name = string(nameData[:nameLen-1])

	return socket, nil
}

func (u *utunSocket) ReadPacket() (packet []byte, err error) {
	err = u.operate("read packet", func() error {
		data := make([]byte, 65536)
		for {
			amount, err := unix.Read(u.fd, data)
			if err == nil {
				packet = data[4:amount]
				return nil
			} else if err != unix.EINTR {
				return err
			}
		}
	})
	return
}

func (u *utunSocket) WritePacket(buffer []byte) (err error) {
	return u.operate("write packet", func() error {
		_, err := syscall.Write(u.fd, append([]byte{0, 0, 0, 2}, buffer...))
		return err
	})
}

func (u *utunSocket) SetAddresses(local, dest net.IP, mask net.IPMask) (err error) {
	return u.operate("set addresses", func() error {
		if local.To4() == nil || dest.To4() == nil || len(mask) != 4 {
			return errors.New("only IPv4 is supported")
		}
		u.ifreqIOCTL(ioctlSIOCDIFADDR, make([]byte, 16*3))
		var sockaddrs bytes.Buffer
		for _, ip := range []net.IP{local, dest, net.IP(mask)} {
			sockaddrs.Write(packSockaddr4(ip, 0))
		}
		return u.ifreqIOCTL(ioctlSIOCAIFADDR, sockaddrs.Bytes())
	})
}

func (u *utunSocket) getControlID() ([]byte, error) {
	// struct ctl_info
	ctlInfo := make([]byte, 128)
	copy(ctlInfo[4:], []byte(utunControl))
	_, _, sysErr := unix.Syscall(unix.SYS_IOCTL, uintptr(u.fd), uintptr(ioctlCTLIOCGINFO),
		uintptr(unsafe.Pointer(&ctlInfo[0])))
	if sysErr != 0 {
		return nil, sysErr
	}
	return ctlInfo[:4], nil
}

func (u *utunSocket) connectToControl(controlID []byte) error {
	// struct sockaddr_ctl
	addrData := make([]byte, 32)
	addrData[0] = 32
	addrData[1] = unix.AF_SYSTEM
	addrData[2] = afSysControl
	copy(addrData[4:], controlID)
	_, _, sysErr := unix.Syscall(unix.SYS_CONNECT, uintptr(u.fd),
		uintptr(unsafe.Pointer(&addrData[0])), uintptr(32))
	if sysErr != 0 {
		return sysErr
	}
	return nil
}

func (u *utunSocket) ifreqIOCTL(ioctl int, reqData []byte) error {
	sock, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}
	defer unix.Close(sock)

	var ifreq []byte
	if len(reqData) > 16 {
		ifreq = make([]byte, 16+len(reqData))
	} else {
		ifreq = make([]byte, 32)
	}
	copy(ifreq[:16], []byte(u.Name()))
	copy(ifreq[16:], reqData)
	_, _, sysErr := unix.Syscall(unix.SYS_IOCTL, uintptr(sock), uintptr(ioctl),
		uintptr(unsafe.Pointer(&ifreq[0])))
	copy(reqData, ifreq[16:])
	if sysErr == 0 {
		return nil
	}
	return sysErr
}
