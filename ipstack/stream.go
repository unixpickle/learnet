package ipstack

import (
	"errors"
	"sync"
)

// A Stream is a bidirectional stream of packets.
//
// A Stream could be attached to anything from a network
// interface to a TCP connection.
//
// Closing a stream should close the incoming channel and
// the done channel.
type Stream interface {
	// Incoming returns a channel of incoming packets.
	//
	// The channel is closed when the stream is closed or
	// disconnected.
	//
	// Received buffers belong to the receiver.
	Incoming() <-chan []byte

	// Outgoing returns a channel for sending packets.
	//
	// Closing this channel to closes the Stream.
	//
	// After sending a buffer, the buffer belongs to the
	// Stream.
	Outgoing() chan<- []byte

	// Done returns a channel which is closed automatically
	// when the Stream is closed or disconnected.
	Done() <-chan struct{}
}

// A MultiStream multiplexes an underlying stream.
type MultiStream interface {
	// Fork creates a Stream that reads/writes to the
	// underlying Stream.
	//
	// The child Stream will provide back-pressure on the
	// parent, meaning that if you don't read packets on the
	// child, packets will stop flowing through the parent.
	Fork() (Stream, error)

	// Close closes the underlying Stream and all the child
	// streams.
	Close() error
}

type multiplexer struct {
	stream     Stream
	lock       sync.Mutex
	children   []*childStream
	bufferSize int
	writeChan  chan<- []byte
	closeChan  chan struct{}
	addChan    chan struct{}
}

// Multiplex creates a MultiStream from a Stream.
//
// The bufferSize is the number of packets can be stored
// in a read/write queue before backpressure takes place.
func Multiplex(stream Stream, bufferSize int) MultiStream {
	writeChan := make(chan []byte, bufferSize)
	res := &multiplexer{
		stream:     stream,
		bufferSize: bufferSize,
		writeChan:  writeChan,
		closeChan:  make(chan struct{}),
		addChan:    make(chan struct{}, 1),
	}
	go res.readLoop()
	go res.writeLoop(writeChan)
	return res
}

func (m *multiplexer) Fork() (Stream, error) {
	child := &childStream{
		rawIncoming: make(chan []byte, m.bufferSize),
		incoming:    make(chan []byte),
		outgoing:    make(chan []byte, m.bufferSize),
		done:        make(chan struct{}),
	}

	m.lock.Lock()
	defer m.lock.Unlock()
	if m.closed() {
		return nil, errors.New("fork MultiStream: stream closed")
	}
	m.children = append(m.children, child)

	go child.outgoingLoop(m.closeChan, m.writeChan)
	go child.incomingLoop()

	select {
	case m.addChan <- struct{}{}:
	default:
	}

	return child, nil
}

func (m *multiplexer) Close() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.closed() {
		return errors.New("already closed")
	}
	close(m.closeChan)

	return nil
}

func (m *multiplexer) readLoop() {
	defer m.Close()
	for {
		packet, ok := <-m.stream.Incoming()
		if !ok {
			return
		}

		var children []*childStream
		for len(children) == 0 {
			m.lock.Lock()
			children = append([]*childStream{}, m.children...)
			m.lock.Unlock()
			if len(children) == 0 {
				select {
				case <-m.addChan:
				case <-m.closeChan:
					return
				case <-m.stream.Done():
					return
				}
			}
		}

		for _, child := range children {
			select {
			case child.rawIncoming <- packet:
			case <-child.done:
			case <-m.stream.Done():
				return
			case <-m.closeChan:
				return
			}
		}
	}
}

func (m *multiplexer) writeLoop(writeChan <-chan []byte) {
	defer close(m.stream.Outgoing())
	for {
		select {
		case packet := <-writeChan:
			select {
			case m.stream.Outgoing() <- packet:
			case <-m.closeChan:
				return
			}
		case <-m.closeChan:
			return
		}
	}
}

func (m *multiplexer) closed() bool {
	select {
	case <-m.closeChan:
		return true
	default:
		return false
	}
}

type childStream struct {
	rawIncoming chan []byte
	incoming    chan []byte
	outgoing    chan []byte
	done        chan struct{}
}

func (c *childStream) Incoming() <-chan []byte {
	return c.incoming
}

func (c *childStream) Outgoing() chan<- []byte {
	return c.outgoing
}

func (c *childStream) Done() <-chan struct{} {
	return c.done
}

func (c *childStream) outgoingLoop(closeChan chan struct{}, writeChan chan<- []byte) {
	defer close(c.done)
	for {
		select {
		case packet, ok := <-c.outgoing:
			if !ok {
				return
			}
			select {
			case writeChan <- packet:
			case <-closeChan:
				return
			}
		case <-closeChan:
			return
		}
	}
}

func (c *childStream) incomingLoop() {
	defer close(c.incoming)
	for {
		select {
		case packet := <-c.rawIncoming:
			select {
			case c.incoming <- packet:
			case <-c.done:
				return
			}
		case <-c.done:
			return
		}
	}
}
