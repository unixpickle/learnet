package ipstack

import (
	"errors"
	"net"
	"sync"
)

// A PortAllocator is a pool of available ports.
type PortAllocator interface {
	// Exclusive use of a port.
	// Useful for incoming connections.
	AllocAny() (int, error)
	Alloc(port int) error
	Free(port int) error

	// Shared use of a dynamically-chosen port.
	// Useful for outgoing connections.
	AllocRemote(remote net.Addr) (int, error)
	FreeRemote(remote net.Addr, port int) error
}

// BasicPortAllocator makes a PortAllocator that uses a
// single pool of 16-bit ports.
func BasicPortAllocator() PortAllocator {
	return &basicPortAllocator{}
}

type basicPortAllocator struct {
	Lock sync.Mutex
	Used [0x10000]bool
}

func (b *basicPortAllocator) AllocAny() (int, error) {
	b.Lock.Lock()
	defer b.Lock.Unlock()
	for i := 0xffff; i >= 0; i-- {
		if !b.Used[i] {
			b.Used[i] = true
			return i, nil
		}
	}
	return 0, errors.New("alloc port: no free ports")
}

func (b *basicPortAllocator) Alloc(port int) error {
	if port < 0 || port >= 0x10000 {
		return errors.New("alloc port: port out of range")
	}
	b.Lock.Lock()
	defer b.Lock.Unlock()
	if !b.Used[port] {
		b.Used[port] = true
		return nil
	}
	return errors.New("alloc port: port in use")
}

func (b *basicPortAllocator) Free(port int) error {
	if port < 0 || port >= 0x10000 {
		return errors.New("free port: port out of range")
	}
	b.Lock.Lock()
	defer b.Lock.Unlock()
	if b.Used[port] {
		b.Used[port] = false
		return nil
	}
	return errors.New("free port: port is not in use")
}

func (b *basicPortAllocator) AllocRemote(remote net.Addr) (int, error) {
	return b.AllocAny()
}

func (b *basicPortAllocator) FreeRemote(remote net.Addr, port int) error {
	return b.Free(port)
}
