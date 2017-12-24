// Package tunnet provides a high-level API for creating
// tunnel network interfaces.
package tunnet

// An IP Tunnel network interface.
//
// Supports sending/receiving IP packets.
type Tunnel interface {
	Name() string
	ReadPacket() ([]byte, error)
	WritePacket([]byte) error

	MTU() (int, error)
	SetMTU(mtu int) error

	Close() error
}
