package ipstack

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
	Incoming() <-chan []byte

	// Outgoing returns a channel for sending packets.
	//
	// Closing this channel to closes the Stream.
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
	Fork(bufferSize int) (Stream, error)

	// Close closes the underlying Stream and all the child
	// streams.
	Close() error
}
