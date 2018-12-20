package ipstack

import (
	"errors"
	"io"
	"net"
	"time"
)

// A TCPNet performs functions for a TCP host.
// In particular, it can create net.Conns for TCP
// connections.
type TCPNet interface {
	DialTCP(addr *net.TCPAddr) (net.Conn, error)
	ListenTCP(addr *net.TCPAddr) (net.Listener, error)
	Close() error
}

type tcp4Net struct {
	stream MultiStream
	laddr  net.IP
	ports  PortAllocator
	ttl    int
}

// NewTCP4Net creates a TCPNet on top of a Stream.
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
func NewTCP4Net(stream Stream, laddr net.IP, ports PortAllocator, ttl int) TCPNet {
	if ports == nil {
		ports = BasicPortAllocator()
	}
	if ttl == 0 {
		ttl = DefaultTTL
	}
	stream = FilterIPv4Proto(stream, ProtocolNumberTCP)
	stream = FilterIPv4Dest(stream, laddr)
	stream = Filter(stream, func(packet []byte) []byte {
		tp := TCP4Packet(packet)
		if tp.Valid() && tp.Checksum() == 0 {
			return packet
		}
		return nil
	}, nil)
	return &tcp4Net{
		stream: Multiplex(stream),
		laddr:  laddr,
		ports:  ports,
		ttl:    ttl,
	}
}

func (t *tcp4Net) DialTCP(addr *net.TCPAddr) (net.Conn, error) {
	return nil, errors.New("not yet implemented")
}

func (t *tcp4Net) ListenTCP(addr *net.TCPAddr) (net.Listener, error) {
	stream, err := t.stream.Fork(16)
	if err != nil {
		return nil, io.ErrClosedPipe
	}
	if err := t.ports.Alloc(addr.Port); err != nil {
		return nil, err
	}
	res := &tcp4Listener{
		stream: Multiplex(stream),
		addr:   addr,
		conns:  make(chan *tcp4Conn, 1),
		ttl:    t.ttl,
		ports:  t.ports,
	}
	go res.loop()
	return res, nil
}

func (t *tcp4Net) Close() error {
	return t.stream.Close()
}

type tcp4Listener struct {
	stream MultiStream
	addr   *net.TCPAddr
	conns  chan *tcp4Conn
	ttl    int
	ports  PortAllocator
}

func (t *tcp4Listener) Accept() (net.Conn, error) {
	conn := <-t.conns
	if conn == nil {
		return nil, io.ErrClosedPipe
	}
	return conn, nil
}

func (t *tcp4Listener) Close() error {
	if err := t.stream.Close(); err != nil {
		return err
	}
	return t.ports.Free(t.addr.Port)
}

func (t *tcp4Listener) Addr() net.Addr {
	return t.addr
}

func (t *tcp4Listener) loop() {
	defer close(t.conns)
	stream, err := t.stream.Fork(10)
	if err != nil {
		return
	}
	stream = filterTCP4Dest(stream, t.addr)
	stream = filterTCP4Syn(stream)
	for packet := range stream.Incoming() {
		tp := TCP4Packet(packet)

		stream, err := t.stream.Fork(10)
		if err != nil {
			return
		}
		stream = filterTCP4Source(stream, tp.SourceAddr())
		stream = filterTCP4Dest(stream, tp.DestAddr())

		handshake, err := tcp4ServerHandshake(stream, tp, t.ttl)
		if err != nil {
			stream.Close()
			return
		}
		conn := &tcp4Conn{
			stream: stream,
			laddr:  tp.DestAddr(),
			raddr:  tp.SourceAddr(),
			recv:   newSimpleTcpRecv(handshake.remoteSeq, 128),
			send:   newSimpleTcpSend(handshake.localSeq, handshake.remoteWinSize, handshake.mss),
			ttl:    t.ttl,
		}
		go conn.loop()
		t.conns <- conn
	}
}

type tcp4Conn struct {
	stream Stream

	laddr *net.TCPAddr
	raddr *net.TCPAddr

	recv tcpRecv
	send tcpSend

	ttl int
}

func (t *tcp4Conn) Read(b []byte) (int, error) {
	return t.recv.Read(b)
}

func (t *tcp4Conn) Write(b []byte) (int, error) {
	return t.send.Write(b)
}

func (t *tcp4Conn) LocalAddr() net.Addr {
	return t.laddr
}

func (t *tcp4Conn) RemoteAddr() net.Addr {
	return t.raddr
}

func (t *tcp4Conn) SetDeadline(d time.Time) error {
	t.SetReadDeadline(d)
	t.SetWriteDeadline(d)
	return nil
}

func (t *tcp4Conn) SetReadDeadline(d time.Time) error {
	t.recv.SetDeadline(d)
	return nil
}

func (t *tcp4Conn) SetWriteDeadline(d time.Time) error {
	t.send.SetDeadline(d)
	return nil
}

func (t *tcp4Conn) Close() error {
	return t.send.Close()
}

func (t *tcp4Conn) loop() {
	defer t.stream.Close()
	for !t.send.Done() || !t.recv.Done() {
		select {
		case outgoing := <-t.send.Next():
			t.sendSegment(outgoing)
		case <-t.recv.WindowOpen():
			t.sendAck()
		case packet := <-t.stream.Incoming():
			tp := TCP4Packet(packet)
			segment := &tcpSegment{
				Start: tp.Header().SeqNum(),
				Data:  tp.Payload(),
				Fin:   tp.Header().Flag(FIN),
			}
			t.recv.Handle(segment)
			t.send.Handle(tp.Header().AckNum(), tp.Header().WindowSize())
			t.sendAck()
		}
	}
}

func (t *tcp4Conn) sendAck() {
	packet := NewTCP4Packet(t.ttl, t.laddr, t.raddr, 0, t.recv.Ack(), t.recv.Window(), nil, ACK)
	select {
	case t.stream.Outgoing() <- packet:
	default:
	}
}

func (t *tcp4Conn) sendSegment(seg *tcpSegment) {
	packet := NewTCP4Packet(t.ttl, t.laddr, t.raddr, seg.Start, t.recv.Ack(), t.recv.Window(),
		seg.Data, ACK)
	if seg.Fin {
		packet.Header().SetFlag(FIN, true)
		packet.SetChecksum()
	}
	select {
	case t.stream.Outgoing() <- packet:
	default:
	}
}

func filterTCP4Dest(s Stream, addr *net.TCPAddr) Stream {
	return Filter(s, func(packet []byte) []byte {
		tp := TCP4Packet(packet)
		if !tp.DestAddr().IP.Equal(addr.IP) || tp.DestAddr().Port != addr.Port {
			return nil
		}
		return packet
	}, nil)
}

func filterTCP4Source(s Stream, addr *net.TCPAddr) Stream {
	return Filter(s, func(packet []byte) []byte {
		tp := TCP4Packet(packet)
		if !tp.SourceAddr().IP.Equal(addr.IP) || tp.SourceAddr().Port != addr.Port {
			return nil
		}
		return packet
	}, nil)
}

func filterTCP4Syn(s Stream) Stream {
	return Filter(s, func(packet []byte) []byte {
		if TCP4Packet(packet).Header().Flag(SYN) {
			return packet
		}
		return nil
	}, nil)
}
