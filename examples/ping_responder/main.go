// Demonstrate the basic setup for IP streams.
//
// Run this as root.

package main

import (
	"fmt"
	"net"

	"github.com/unixpickle/essentials"
	"github.com/unixpickle/learnet/ipstack"
	"github.com/unixpickle/learnet/tunnet"
)

const BufferSize = 10

func main() {
	tun, err := tunnet.MakeTunnel()
	essentials.Must(err)

	mtu, err := tun.MTU()
	essentials.Must(err)

	myIP := net.ParseIP("10.13.37.2")
	routerIP := net.ParseIP("10.13.37.1")
	netmask := net.IPMask{255, 255, 255, 252}
	essentials.Must(tun.SetAddresses(myIP, routerIP, netmask))
	essentials.Must(tunnet.AddRoute(routerIP, routerIP, netmask))

	stream := tunnet.TunnelStream(tun, BufferSize, BufferSize)
	stream = ipstack.FilterIPv4Valid(stream)
	stream = ipstack.FilterIPv4Checksums(stream)
	stream = ipstack.DefragmentIncomingIPv4(stream, 0)
	stream = ipstack.FragmentOutgoingIPv4(stream, mtu)
	stream = ipstack.AddIPv4Identifiers(stream)

	multi := ipstack.Multiplex(stream)

	// Loopback lets the user ping themselves.
	stream1, err := multi.Fork(BufferSize)
	essentials.Must(err)
	go ipstack.Loopback(ipstack.FilterIPv4Dest(stream1, myIP))

	fmt.Printf("Try pinging yourself (%s) or the gateway (%s)!\n", myIP, routerIP)

	// Respond to pings for the gateway.
	stream2, err := multi.Fork(BufferSize)
	essentials.Must(err)
	ipstack.RespondToPingsIPv4(ipstack.FilterIPv4Dest(stream2, routerIP))
}
