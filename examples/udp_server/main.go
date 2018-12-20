// Demonstrate the basic setup for UDP servers.
//
// Run this as root.

package main

import (
	"fmt"
	"net"

	"github.com/unixpickle/essentials"
	"github.com/unixpickle/learnet/examples"
	"github.com/unixpickle/learnet/ipstack"
)

const BufferSize = 10

func main() {
	multi := examples.SetupTunnel()
	examples.SetupIPServices(multi)

	udpStream, err := multi.Fork(BufferSize)
	essentials.Must(err)
	udpNet := ipstack.NewUDP4Net(udpStream, examples.Gateway, nil, 0, 0)

	listener, err := udpNet.ListenUDP(&net.UDPAddr{Port: 1337})
	essentials.Must(err)

	go func() {
		for {
			packet := make([]byte, 0x10000)
			n, addr, err := listener.ReadFrom(packet)
			essentials.Must(err)
			fmt.Printf("got %d bytes from %s: %s\n", n, addr, string(packet[:n]))
			_, err = listener.WriteTo([]byte("got: "+string(packet[:n])), addr)
			essentials.Must(err)
		}
	}()

	fmt.Printf("Send UDP traffic to %s:1337!\n", examples.Gateway)

	select {}
}
