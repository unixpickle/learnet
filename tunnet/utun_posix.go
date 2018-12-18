//+build darwin linux

package tunnet

import (
	"net"
	"os"
	"sync"
	"syscall"

	"github.com/unixpickle/essentials"
)

type fdTunnelBase struct {
	fd   int
	name string

	refLock  sync.Mutex
	refCount int
	closed   bool
}

func (f *fdTunnelBase) Name() string {
	return f.name
}

func (f *fdTunnelBase) Close() (err error) {
	return f.operate("close", func() error {
		err := syscall.Shutdown(f.fd, syscall.SHUT_RDWR)
		f.refLock.Lock()
		f.closed = true
		f.refLock.Unlock()
		return err
	})
}

func (f *fdTunnelBase) operate(ctx string, fu func() error) error {
	f.refLock.Lock()
	if f.closed {
		f.refLock.Unlock()
		return essentials.AddCtx(ctx, os.ErrClosed)
	}
	f.refCount += 1
	f.refLock.Unlock()

	defer func() {
		f.refLock.Lock()
		defer f.refLock.Unlock()
		f.refCount -= 1
		if f.closed && f.refCount == 0 {
			syscall.Close(f.fd)
		}
	}()

	return essentials.AddCtx(ctx, fu())
}

type bareFdTunnel interface {
	Name() string
	ReadPacket() (packet []byte, err error)
	WritePacket(buffer []byte) (err error)
	SetAddresses(local, dest net.IP, mask net.IPMask) (err error)
	Close() (err error)

	ifreqIOCTL(ioctl int, reqData []byte) error
	operate(ctx string, f func() error) error
}

type posixTunnel struct {
	bareFdTunnel
}

func (p *posixTunnel) MTU() (mtu int, err error) {
	err = p.operate("get MTU", func() error {
		buf := make([]byte, 4)
		if err := p.ifreqIOCTL(syscall.SIOCGIFMTU, buf); err != nil {
			return err
		}
		mtu = int(systemByteOrder.Uint32(buf))
		return nil
	})
	return
}

func (p *posixTunnel) SetMTU(mtu int) (err error) {
	return p.operate("set MTU", func() error {
		var buf [4]byte
		systemByteOrder.PutUint32(buf[:], uint32(mtu))
		return p.ifreqIOCTL(syscall.SIOCSIFMTU, buf[:])
	})
}

func (p *posixTunnel) Addresses() (local, dest net.IP, mask net.IPMask, err error) {
	err = p.operate("get addresses", func() error {
		var maskIP net.IP
		outputs := []*net.IP{&local, &dest, &maskIP}
		ioctls := []int{syscall.SIOCGIFADDR, syscall.SIOCGIFDSTADDR, syscall.SIOCGIFNETMASK}
		for i, ioctl := range ioctls {
			sockaddrOut := packSockaddr4(net.IPv4zero, 0)
			if err := p.ifreqIOCTL(ioctl, sockaddrOut); err != nil {
				return err
			}
			*outputs[i], _ = unpackSockaddr4(sockaddrOut)
		}
		mask = net.IPMask(maskIP)
		return nil
	})
	return
}
