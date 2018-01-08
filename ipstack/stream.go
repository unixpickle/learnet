package ipstack

import (
	"errors"
	"sync"

	"github.com/unixpickle/essentials"
)

var (
	AlreadyClosedErr = errors.New("close: stream is closed")
	WriteClosedErr   = errors.New("write: stream is closed")
)

const DefaultBufferSize = 100

// A Stream is a bidirectional stream of packets.
//
// Streams are non-blocking and can never apply
// backpressure to a source or a sender.
// Thus, you should only use a Stream to represent a
// connection that allows for packet loss.
//
// When a stream closes (either due to a Close() or a
// disconnect), the Incoming() and Done() channels are
// closed.
type Stream interface {
	// Incoming returns the channel of incoming packets.
	//
	// The channel is closed when the stream is closed or
	// disconnected.
	//
	// Received buffers belong to the receiver.
	Incoming() <-chan []byte

	// Outgoing returns the channel to which outgoing packets
	// can be sent.
	//
	// Outgoing writes may block due to backpressure from the
	// network stack or the actual network hardware.
	// Also, outgoing writes may block forever once the
	// Stream has been closed.
	// Thus, it is important to select on both the Outgoing()
	// channel and the Done() channel.
	//
	// Sent buffers belong to the Stream.
	//
	// This channel should never be closed.
	Outgoing() chan<- []byte

	// Close shuts down the Stream.
	Close() error

	// Done returns a channel which is closed automatically
	// when the Stream is closed or disconnected.
	Done() <-chan struct{}
}

// Send writes a packet to the stream or returns with an
// error if the stream is closed.
func Send(s Stream, data []byte) error {
	select {
	case <-s.Done():
		return WriteClosedErr
	default:
	}
	select {
	case s.Outgoing() <- data:
	case <-s.Done():
		return WriteClosedErr
	}
	return nil
}

// Loopback writes all received packets on a Stream back
// to the same stream.
// It blocks until the Stream is closed.
//
// This is useful for tunnel interfaces, where a host
// expects to be able to reach itself through a tunnel.
// In that case, you should feed Loopback()) a Stream
// which has been filtered for the host's IP address.
func Loopback(stream Stream) {
	for packet := range stream.Incoming() {
		select {
		case stream.Outgoing() <- packet:
		case <-stream.Done():
			return
		}
	}
}

// A MultiStream multiplexes an underlying stream.
type MultiStream interface {
	// Fork creates a Stream that reads/writes to the
	// underlying Stream.
	//
	// The child Stream will buffer up to readBuffer packets,
	// after which point packets will be dropped.
	Fork(readBuffer int) (Stream, error)

	// Close closes the underlying Stream and all the child
	// streams.
	Close() error
}

type pipeStream struct {
	other *pipeStream

	pairLock *sync.Mutex

	closed   bool
	incoming chan []byte
	outgoing chan []byte
	done     chan struct{}
}

// Pipe creates two connected Streams.
//
// Each stream applies backpressure on the other.
// If you don't read from one stream for a while, then
// writes to the other stream will block.
// However, you may specify a buffer size, in which case
// at least bufferSize writes will be buffered.
func Pipe(bufferSize int) (Stream, Stream) {
	right := make(chan []byte, bufferSize)
	left := make(chan []byte, bufferSize)
	done := make(chan struct{})
	lock := &sync.Mutex{}

	pipe1 := &pipeStream{
		pairLock: lock,
		incoming: make(chan []byte),
		outgoing: make(chan []byte),
		done:     done,
	}
	pipe2 := &pipeStream{
		pairLock: lock,
		incoming: make(chan []byte),
		outgoing: make(chan []byte),
		done:     done,
	}
	pipe1.other = pipe2
	pipe2.other = pipe1
	go forwardPackets(pipe1.incoming, left, true, done)
	go forwardPackets(pipe2.incoming, right, true, done)
	go forwardPackets(right, pipe1.outgoing, false, done)
	go forwardPackets(left, pipe2.outgoing, false, done)

	return pipe1, pipe2
}

func (p *pipeStream) Incoming() <-chan []byte {
	return p.incoming
}

func (p *pipeStream) Outgoing() chan<- []byte {
	return p.outgoing
}

func (p *pipeStream) Close() error {
	p.pairLock.Lock()
	defer p.pairLock.Unlock()

	if p.closed {
		return AlreadyClosedErr
	}
	p.closed = true
	if !p.other.closed {
		close(p.done)
	}
	return nil
}

func (p *pipeStream) Done() <-chan struct{} {
	return p.done
}

type multiplexer struct {
	stream    Stream
	lock      sync.Mutex
	children  []*childStream
	closeChan chan struct{}
}

// Multiplex creates a MultiStream from a Stream.
func Multiplex(stream Stream) MultiStream {
	res := &multiplexer{
		stream:    stream,
		closeChan: make(chan struct{}),
	}
	go res.readLoop()
	return res
}

func (m *multiplexer) Fork(readBuffer int) (Stream, error) {
	child := &childStream{
		parent:   m,
		incoming: make(chan []byte, readBuffer),
		outgoing: make(chan []byte),
		done:     make(chan struct{}),
	}
	go forwardPackets(m.stream.Outgoing(), child.outgoing, false, child.done, m.closeChan)

	m.lock.Lock()
	defer m.lock.Unlock()
	if m.closed() {
		return nil, errors.New("fork MultiStream: stream closed")
	}
	m.children = append(m.children, child)

	return child, nil
}

func (m *multiplexer) Close() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.closed() {
		return AlreadyClosedErr
	}

	close(m.closeChan)
	m.stream.Close()

	for _, child := range m.children {
		close(child.done)
		close(child.incoming)
	}

	return nil
}

func (m *multiplexer) readLoop() {
	defer m.Close()
	for {
		select {
		case packet := <-m.stream.Incoming():
			if !m.handleIncoming(packet) {
				return
			}
		case <-m.stream.Done():
			return
		case <-m.closeChan:
			return
		}
	}
}

func (m *multiplexer) handleIncoming(packet []byte) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.closed() {
		return false
	}

	for _, child := range m.children {
		select {
		case child.incoming <- append([]byte{}, packet...):
		default:
		}
	}

	return true
}

func (m *multiplexer) closed() bool {
	select {
	case <-m.closeChan:
		return true
	default:
		return false
	}
}

func (m *multiplexer) childClose(child *childStream) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	for i, ch := range m.children {
		if ch == child {
			essentials.UnorderedDelete(&m.children, i)
			close(child.done)
			close(child.incoming)
			return nil
		}
	}

	return AlreadyClosedErr
}

type childStream struct {
	parent   *multiplexer
	incoming chan []byte
	outgoing chan []byte
	done     chan struct{}
}

func (c *childStream) Incoming() <-chan []byte {
	return c.incoming
}

func (c *childStream) Outgoing() chan<- []byte {
	return c.outgoing
}

func (c *childStream) Close() error {
	return c.parent.childClose(c)
}

func (c *childStream) Done() <-chan struct{} {
	return c.done
}
