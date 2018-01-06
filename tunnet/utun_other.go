// +build !darwin

package tunnet

import "errors"

// MakeTunnel creates a new tunnel interface.
func MakeTunnel() (Tunnel, error) {
	return nil, errors.New("make tunnel: not supported on this platform")
}
