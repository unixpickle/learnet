package examples

import (
	"net"

	"github.com/unixpickle/essentials"
	"github.com/unixpickle/learnet/ipstack"
	"github.com/unixpickle/learnet/tunnet"
)

const BufferSize = 10

var (
	MyIP    = net.ParseIP("10.13.37.2")
	Gateway = net.ParseIP("10.13.37.1")
	Netmask = net.IPMask{255, 255, 255, 252}
)

// SetupTunnel creates a tunnel interface for a demo.
func SetupTunnel() ipstack.MultiStream {
	tun, err := tunnet.MakeTunnel()
	essentials.Must(err)

	mtu, err := tun.MTU()
	essentials.Must(err)

	essentials.Must(tun.SetAddresses(MyIP, Gateway, Netmask))
	essentials.Must(tunnet.AddRoute(Gateway, Gateway, Netmask))

	stream := tunnet.TunnelStream(tun, BufferSize, BufferSize)
	stream = ipstack.FilterIPv4Valid(stream)
	stream = ipstack.FilterIPv4Checksums(stream)
	stream = ipstack.DefragmentIncomingIPv4(stream, 0)
	stream = ipstack.FragmentOutgoingIPv4(stream, mtu)
	stream = ipstack.AddIPv4Identifiers(stream)

	return ipstack.Multiplex(stream)
}

// SetupIPServices starts a ping responder and loopback
// handler for a tunnel created by SetupTunnel().
func SetupIPServices(tunnel ipstack.MultiStream) {
	stream1, err := tunnel.Fork(BufferSize)
	essentials.Must(err)
	go ipstack.Loopback(ipstack.FilterIPv4Dest(stream1, MyIP))

	stream2, err := tunnel.Fork(BufferSize)
	essentials.Must(err)
	go ipstack.RespondToPingsIPv4(ipstack.FilterIPv4Dest(stream2, Gateway))
}
