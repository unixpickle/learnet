// +build linux

package tunnet

import (
	"errors"
	"net"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"github.com/unixpickle/essentials"
)

const tunDev = "/dev/net/tun"

// MakeTunnel creates a new tunnel interface.
func MakeTunnel() (Tunnel, error) {
	tun, err := openTunFile()
	err = essentials.AddCtx("make tunnel", err)
	return tun, err
}

type tunFile struct {
	fd   int
	name string

	refLock  sync.Mutex
	refCount int
	closed   bool
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
		fd:   fd,
		name: string(ifreq[:nameLen]),
	}

	var flags [2]byte
	if err := res.ifreqIOCTL(syscall.SIOCGIFFLAGS, flags[:]); err != nil {
		syscall.Close(fd)
		return nil, err
	}
	var newFlags [2]byte
	// Set IFF_UP.
	systemByteOrder.PutUint16(newFlags[:], 1)
	flags[0] |= newFlags[0]
	flags[1] |= newFlags[1]
	if err := res.ifreqIOCTL(syscall.SIOCSIFFLAGS, flags[:]); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	return res, nil
}

func (t *tunFile) Name() string {
	return t.name
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

func (t *tunFile) MTU() (mtu int, err error) {
	err = t.operate("get MTU", func() error {
		buf := make([]byte, 4)
		if err := t.ifreqIOCTL(syscall.SIOCGIFMTU, buf); err != nil {
			return err
		}
		mtu = int(systemByteOrder.Uint32(buf))
		return nil
	})
	return
}

func (t *tunFile) SetMTU(mtu int) (err error) {
	return t.operate("set MTU", func() error {
		var buf [4]byte
		systemByteOrder.PutUint32(buf[:], uint32(mtu))
		return t.ifreqIOCTL(syscall.SIOCSIFMTU, buf[:])
	})
}

func (t *tunFile) Addresses() (local, dest net.IP, mask net.IPMask, err error) {
	err = t.operate("get addresses", func() error {
		var maskIP net.IP
		outputs := []*net.IP{&local, &dest, &maskIP}
		ioctls := []int{syscall.SIOCGIFADDR, syscall.SIOCGIFDSTADDR, syscall.SIOCGIFNETMASK}
		for i, ioctl := range ioctls {
			sockaddrOut := packSockaddr4(net.IPv4zero, 0)
			if err := t.ifreqIOCTL(ioctl, sockaddrOut); err != nil {
				return err
			}
			*outputs[i], _ = unpackSockaddr4(sockaddrOut)
		}
		mask = net.IPMask(maskIP)
		return nil
	})
	return
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

func (t *tunFile) Close() (err error) {
	return t.operate("close", func() error {
		err := syscall.Shutdown(t.fd, syscall.SHUT_RDWR)
		t.refLock.Lock()
		t.closed = true
		t.refLock.Unlock()
		return err
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

func (t *tunFile) operate(ctx string, f func() error) error {
	t.refLock.Lock()
	if t.closed {
		t.refLock.Unlock()
		return essentials.AddCtx(ctx, os.ErrClosed)
	}
	t.refCount += 1
	t.refLock.Unlock()

	defer func() {
		t.refLock.Lock()
		defer t.refLock.Unlock()
		t.refCount -= 1
		if t.closed && t.refCount == 0 {
			syscall.Close(t.fd)
		}
	}()

	return essentials.AddCtx(ctx, f())
}
