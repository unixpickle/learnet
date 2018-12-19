package ipstack

import (
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
}

type tcp4Listener struct {
	stream MultiStream
	addr   *net.TCPAddr
	conns  chan *tcp4Conn
}

func (t *tcp4Listener) Accept() (net.Conn, error) {
	conn := <-t.conns
	if conn == nil {
		return nil, io.ErrClosedPipe
	}
	return conn, nil
}

func (t *tcp4Listener) Close() error {
	return t.stream.Close()
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

		// TODO: negotiate connection here.

		conn := &tcp4Conn{
			stream: stream,
			laddr:  tp.DestAddr(),
			raddr:  tp.SourceAddr(),
			recv:   newSimpleTcpRecv(1337, 128),
			send:   newSimpleTcpSend(1337, 128, 128),
		}
		t.conns <- conn
	}
}

type tcp4Conn struct {
	stream Stream

	laddr *net.TCPAddr
	raddr *net.TCPAddr

	recv tcpRecv
	send tcpSend
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
	// TODO: this.
}

func (t *tcp4Conn) sendSegment(seg *tcpSegment) {
	// TODO: this.
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
