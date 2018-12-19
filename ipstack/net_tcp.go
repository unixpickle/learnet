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

type tcpNet struct {
	stream MultiStream
}

type tcpListener struct {
	stream MultiStream
	addr   *net.TCPAddr
	conns  chan *tcpConn
}

func (t *tcpListener) Accept() (net.Conn, error) {
	conn := <-t.conns
	if conn == nil {
		return nil, io.ErrClosedPipe
	}
	return conn, nil
}

func (t *tcpListener) Close() error {
	return t.stream.Close()
}

func (t *tcpListener) Addr() net.Addr {
	return t.addr
}

func (t *tcpListener) loop() {
	defer close(t.conns)
	stream, err := t.stream.Fork(10)
	if err != nil {
		return
	}
	stream = Filter(stream, func(packet []byte) []byte {
		tp := TCP4Packet(packet)
		if !tp.DestAddr().IP.Equal(t.addr.IP) || tp.DestAddr().Port != t.addr.Port {
			return nil
		}
		if !tp.Header().Flag(SYN) {
			return nil
		}
		return packet
	}, nil)
	for packet := range stream.Incoming() {
		tp := TCP4Packet(packet)

		stream, err := t.stream.Fork(10)
		if err != nil {
			return
		}
		raddr := tp.DestAddr()
		stream = Filter(stream, func(packet []byte) []byte {
			newRaddr := TCP4Packet(packet).DestAddr()
			if newRaddr.IP.Equal(raddr.IP) && newRaddr.Port == raddr.Port {
				return packet
			}
			return nil
		}, nil)

		// TODO: negotiate connection here.

		conn := &tcpConn{
			stream: stream,
			laddr:  t.addr,
			raddr:  raddr,
			recv:   newSimpleTcpRecv(1337, 128),
			send:   newSimpleTcpSend(1337, 128, 128),
		}
		t.conns <- conn
	}
}

type tcpConn struct {
	stream Stream

	laddr *net.TCPAddr
	raddr *net.TCPAddr

	recv tcpRecv
	send tcpSend
}

func (t *tcpConn) Read(b []byte) (int, error) {
	return t.recv.Read(b)
}

func (t *tcpConn) Write(b []byte) (int, error) {
	return t.send.Write(b)
}

func (t *tcpConn) LocalAddr() net.Addr {
	return t.laddr
}

func (t *tcpConn) RemoteAddr() net.Addr {
	return t.raddr
}

func (t *tcpConn) SetDeadline(d time.Time) error {
	t.SetReadDeadline(d)
	t.SetWriteDeadline(d)
	return nil
}

func (t *tcpConn) SetReadDeadline(d time.Time) error {
	t.recv.SetDeadline(d)
	return nil
}

func (t *tcpConn) SetWriteDeadline(d time.Time) error {
	t.send.SetDeadline(d)
	return nil
}

func (t *tcpConn) Close() error {
	return t.send.Close()
}

func (t *tcpConn) loop() {
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

func (t *tcpConn) sendAck() {
	// TODO: this.
}

func (t *tcpConn) sendSegment(seg *tcpSegment) {
	// TODO: this.
}
