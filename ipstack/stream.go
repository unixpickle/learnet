package ipstack

import (
	"errors"
	"sync"

	"github.com/unixpickle/essentials"
)

var (
	AlreadyClosedErr   = errors.New("close: stream is closed")
	WriteBufferFullErr = errors.New("write: buffer is full")
	WriteClosedErr     = errors.New("write: stream is closed")
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

	// Write sends a packet to the Stream in a non-blocking
	// manner.
	//
	// After sending a buffer, the buffer belongs to the
	// Stream, even if the write fails.
	Write([]byte) error

	// Close shuts down the Stream.
	Close() error

	// Done returns a channel which is closed automatically
	// when the Stream is closed or disconnected.
	Done() <-chan struct{}
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
// The rightBuffer determines the buffer size when writing
// to s1, whereas the leftBuffer determines the buffer
// size when writing to s2.
func Pipe(rightBuffer, leftBuffer int) (s1, s2 Stream) {
	right := make(chan []byte, rightBuffer)
	left := make(chan []byte, leftBuffer)
	done := make(chan struct{})
	lock := &sync.Mutex{}

	pipe1 := &pipeStream{pairLock: lock, incoming: left, outgoing: right, done: done}
	pipe2 := &pipeStream{pairLock: lock, incoming: right, outgoing: left, done: done}
	pipe1.other = pipe2
	pipe2.other = pipe1

	return pipe1, pipe2
}

func (p *pipeStream) Incoming() <-chan []byte {
	return p.incoming
}

func (p *pipeStream) Write(packet []byte) error {
	p.pairLock.Lock()
	defer p.pairLock.Unlock()

	if p.closed || p.other.closed {
		return WriteClosedErr
	}

	select {
	case p.outgoing <- packet:
		return nil
	default:
		return WriteBufferFullErr
	}
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
		close(p.incoming)
		close(p.outgoing)
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
		done:     make(chan struct{}),
	}

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

func (m *multiplexer) childWrite(child *childStream, data []byte) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Make sure the child isn't closed.
	for _, ch := range m.children {
		if ch == child {
			return m.stream.Write(data)
		}
	}

	return WriteClosedErr
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
	done     chan struct{}
}

func (c *childStream) Incoming() <-chan []byte {
	return c.incoming
}

func (c *childStream) Write(data []byte) error {
	return c.parent.childWrite(c, data)
}

func (c *childStream) Close() error {
	return c.parent.childClose(c)
}

func (c *childStream) Done() <-chan struct{} {
	return c.done
}
