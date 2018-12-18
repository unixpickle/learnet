// +build !darwin,!linux

package tunnet

import (
	"errors"
	"net"
)

// AddRoute adds an entry to the system routing table.
func AddRoute(destination, gateway net.IP, mask net.IPMask) error {
	return errors.New("add route: not supported on this platform")
}
