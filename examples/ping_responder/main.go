// Demonstrate the basic setup for IP streams.
//
// Run this as root.

package main

import (
	"fmt"

	"github.com/unixpickle/learnet/examples"
)

func main() {
	multi := examples.SetupTunnel()
	examples.SetupIPServices(multi)

	fmt.Printf("Try pinging yourself (%s) or the gateway (%s)!\n", examples.MyIP, examples.Gateway)
	select {}
}
