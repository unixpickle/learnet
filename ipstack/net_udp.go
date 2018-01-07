package ipstack

import (
	"errors"
	"net"

	"github.com/unixpickle/essentials"
)

// A UDPConn is an abstract UDP socket.
// It implements both net.Conn and net.PacketConn.
type UDPConn interface {
	net.Conn

	ReadFrom(b []byte) (n int, addr net.Addr, err error)
	WriteTo(b []byte, addr net.Addr) (n int, err error)
}

type UDPNet interface {
	DialUDP(laddr, raddr *net.UDPAddr) (UDPConn, error)
	ListenUDP(laddr *net.UDPAddr) (UDPConn, error)
	Close() error
}

type udp4Net struct {
	multi      MultiStream
	laddr      net.IP
	ports      PortAllocator
	ttl        int
	readBuffer int
}

// NewUDP4Net creates a UDPNet on top of a Stream.
//
// The stream is an IPv4 stream that should automatically
// filter out invalid packets and perform fragmentation.
//
// The laddr is the local address for this network.
// Only packets intended for laddr are processed by the
// network.
//
// The ports argument is used to allocate ports.
// If nil, BasicPortAllocator() is used.
//
// The ttl argument is used as the TTL field for all
// outgoing packets.
// If 0, DefaultTTL is used.
//
// The readBuf argument is the packet read buffer size.
// If 0, DefaultUDPReadBuffer is used.
func NewUDP4Net(stream Stream, laddr net.IP, ports PortAllocator, ttl, readBuf int) UDPNet {
	if ports == nil {
		ports = BasicPortAllocator()
	}
	if ttl == 0 {
		ttl = DefaultTTL
	}
	if readBuf == 0 {
		readBuf = DefaultUDPReadBuffer
	}
	stream = FilterIPv4Proto(stream, ProtocolNumberUDP)
	stream = FilterIPv4Dest(stream, laddr)
	stream = Filter(stream, func(packet []byte) []byte {
		pack := UDP4Packet(packet)
		if pack.Valid() && (!pack.UseChecksum() || pack.Checksum() == 0) {
			return packet
		}
		return nil
	}, nil)
	return &udp4Net{
		multi:      Multiplex(stream),
		laddr:      laddr,
		ports:      ports,
		ttl:        ttl,
		readBuffer: readBuf,
	}
}

func (u *udp4Net) DialUDP(laddr, raddr *net.UDPAddr) (conn UDPConn, err error) {
	defer essentials.AddCtxTo("dial UDP", &err)

	stream, err := u.multi.Fork(u.readBuffer)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			stream.Close()
		}
	}()

	if raddr.IP == nil || raddr.IP.IsUnspecified() {
		raddr.IP = u.laddr
	}
	if laddr == nil {
		laddr = &net.UDPAddr{IP: u.laddr}
		if laddr.Port, err = u.ports.AllocRemote(raddr); err != nil {
			return nil, err
		}
		go func() {
			<-stream.Done()
			u.ports.FreeRemote(raddr, laddr.Port)
		}()
	}

	if !laddr.IP.Equal(u.laddr) {
		return nil, errors.New("cannot listen on address: " + laddr.String())
	}

	filtered := Filter(stream, func(d []byte) []byte {
		source := UDP4Packet(d).SourceAddr()
		dest := UDP4Packet(d).DestAddr()
		if !source.IP.Equal(raddr.IP) || source.Port != raddr.Port || dest.Port != laddr.Port {
			return nil
		}
		return d
	}, nil)
	return &udp4Conn{
		streamConn: newStreamConn(filtered),
		remote:     raddr,
		local:      laddr,
		ttl:        u.ttl,
	}, nil
}

func (u *udp4Net) ListenUDP(laddr *net.UDPAddr) (conn UDPConn, err error) {
	defer essentials.AddCtxTo("listen UDP", &err)

	stream, err := u.multi.Fork(u.readBuffer)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			stream.Close()
		}
	}()

	if laddr == nil {
		laddr = &net.UDPAddr{IP: u.laddr}
		if laddr.Port, err = u.ports.AllocAny(); err != nil {
			return nil, err
		}
		go func() {
			<-stream.Done()
			u.ports.Free(laddr.Port)
		}()
	}
	if laddr.IP == nil || laddr.IP.IsUnspecified() {
		laddr.IP = u.laddr
	}

	if !laddr.IP.Equal(u.laddr) {
		return nil, errors.New("cannot listen on address: " + laddr.String())
	}

	filtered := Filter(stream, func(d []byte) []byte {
		dest := UDP4Packet(d).DestAddr()
		if dest.Port != laddr.Port {
			return nil
		}
		return d
	}, nil)
	return &udp4Conn{
		streamConn: newStreamConn(filtered),
		remote:     nil,
		local:      laddr,
		ttl:        u.ttl,
	}, nil
}

func (u *udp4Net) Close() error {
	return u.multi.Close()
}

type udp4Conn struct {
	*streamConn
	remote *net.UDPAddr
	local  *net.UDPAddr
	ttl    int
}

func (u *udp4Conn) Read(b []byte) (n int, err error) {
	defer essentials.AddCtxTo("read", &err)
	n, _, err = u.ReadFrom(b)
	return
}

func (u *udp4Conn) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	defer essentials.AddCtxTo("read from", &err)
	packet, err := u.streamConn.ReadPacket()
	if err != nil {
		return 0, nil, err
	}
	uPacket := UDP4Packet(packet)
	return copy(b, uPacket.Payload()), uPacket.SourceAddr(), nil
}

func (u *udp4Conn) Write(b []byte) (n int, err error) {
	defer essentials.AddCtxTo("write", &err)
	if u.remote == nil {
		return 0, errors.New("remote is not specified")
	}
	return u.WriteTo(b, u.remote)
}

func (u *udp4Conn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
	defer essentials.AddCtxTo("write to", &err)
	uAddr, ok := addr.(*net.UDPAddr)
	if !ok || uAddr.IP.To4() == nil {
		return 0, errors.New("invalid destination address")
	}
	pack := NewUDP4Packet(u.ttl, u.local, uAddr, b)
	if err := u.streamConn.WritePacket(pack); err != nil {
		return 0, err
	} else {
		return len(b), nil
	}
}

func (u *udp4Conn) LocalAddr() net.Addr {
	return u.local
}

func (u *udp4Conn) RemoteAddr() net.Addr {
	return u.remote
}
