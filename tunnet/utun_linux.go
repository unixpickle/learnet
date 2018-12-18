// +build linux

package tunnet

import (
	"errors"
	"net"
	"os"
	"syscall"
	"unsafe"

	"github.com/unixpickle/essentials"
)

const tunDev = "/dev/net/tun"

// MakeTunnel creates a new tunnel interface.
func MakeTunnel() (Tunnel, error) {
	tun, err := openTunFile()
	err = essentials.AddCtx("make tunnel", err)
	return &posixTunnel{tun}, err
}

type tunFile struct {
	fdTunnelBase
}

func openTunFile() (*tunFile, error) {
	fd, err := syscall.Open(tunDev, os.O_RDWR, 0755)
	if err != nil {
		return nil, err
	}
	ifreq := make([]byte, 40)
	// Set IFF_TUN | IFF_NO_PI
	systemByteOrder.PutUint16(ifreq[16:18], 0x1001)

	_, _, sysErr := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TUNSETIFF,
		uintptr(unsafe.Pointer(&ifreq[0])))
	if sysErr != 0 {
		syscall.Close(fd)
		return nil, sysErr
	}

	_, _, sysErr = syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TUNSETPERSIST,
		0)
	if sysErr != 0 {
		syscall.Close(fd)
		return nil, sysErr
	}

	nameLen := 16
	for i, x := range ifreq[:16] {
		if x == 0 {
			nameLen = i
			break
		}
	}
	res := &tunFile{
		fdTunnelBase{
			fd:   fd,
			name: string(ifreq[:nameLen]),
		},
	}

	var flags [2]byte
	if err := res.ifreqIOCTL(syscall.SIOCGIFFLAGS, flags[:]); err != nil {
		syscall.Close(fd)
		return nil, err
	}
	var newFlags [2]byte
	// Set IFF_Ut.
	systemByteOrder.PutUint16(newFlags[:], 1)
	flags[0] |= newFlags[0]
	flags[1] |= newFlags[1]
	if err := res.ifreqIOCTL(syscall.SIOCSIFFLAGS, flags[:]); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	return res, nil
}

func (t *tunFile) ReadPacket() (packet []byte, err error) {
	err = t.operate("read packet", func() error {
		data := make([]byte, 65536)
		for {
			amount, err := syscall.Read(t.fd, data)
			if err == nil {
				packet = data[:amount]
				return nil
			} else if err != syscall.EINTR {
				return err
			}
		}
	})
	return
}

func (t *tunFile) WritePacket(buffer []byte) (err error) {
	return t.operate("write packet", func() error {
		_, err = syscall.Write(t.fd, buffer)
		return err
	})
}

func (t *tunFile) SetAddresses(local, dest net.IP, mask net.IPMask) (err error) {
	return t.operate("set addresses", func() error {
		if local.To4() == nil || dest.To4() == nil || len(mask) != 4 {
			return errors.New("only IPv4 is supported")
		}
		inputs := []net.IP{local, dest, net.IP(mask)}
		ioctls := []int{syscall.SIOCSIFADDR, syscall.SIOCSIFDSTADDR, syscall.SIOCSIFNETMASK}
		for i, ioctl := range ioctls {
			addr := inputs[i]
			sockaddr := packSockaddr4(addr, 0)
			if err := t.ifreqIOCTL(ioctl, sockaddr); err != nil {
				return err
			}
		}
		return nil
	})
}

func (t *tunFile) ifreqIOCTL(ioctl int, reqData []byte) error {
	sock, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(sock)

	ifreq := make([]byte, 40)
	copy(ifreq, []byte(t.name))
	copy(ifreq[16:], reqData)
	_, _, sysErr := syscall.Syscall(syscall.SYS_IOCTL, uintptr(sock), uintptr(ioctl),
		uintptr(unsafe.Pointer(&ifreq[0])))
	copy(reqData, ifreq[16:])
	if sysErr != 0 {
		return sysErr
	}
	return nil
}
