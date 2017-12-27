package ipstack

import (
	"bytes"
	"testing"
)

func TestMultiplexerBasic(t *testing.T) {
	stream := newTestingStream(10)
	multi := Multiplex(stream, 10)
	defer multi.Close()

	stream1, err := multi.Fork()
	if err != nil {
		t.Error(err)
	}
	stream2, err := multi.Fork()
	if err != nil {
		t.Error(err)
	}

	stream1.Outgoing() <- []byte("test")
	if !bytes.Equal(<-stream.outgoing, []byte("test")) {
		t.Error("unexpected packet")
	}

	stream2.Outgoing() <- []byte("toast")
	if !bytes.Equal(<-stream.outgoing, []byte("toast")) {
		t.Error("unexpected packet")
	}

	stream.incoming <- []byte("foo")
	stream.incoming <- []byte("bar")

	for _, child := range []Stream{stream1, stream2} {
		for _, data := range [][]byte{[]byte("foo"), []byte("bar")} {
			if !bytes.Equal(<-child.Incoming(), data) {
				t.Error("unexpected packet")
			}
		}
	}
}

// TODO: test reading packets from multistream before
// any forks have been made.

// TODO: test closing parent MultiStream.

// TODO: test closing parent MultiStream while a write is
// blocked by backpressure.

// TODO: test closing child stream that is blocking the
// parent.

type testingStream struct {
	incoming chan []byte
	outgoing chan []byte
	done     chan struct{}
}

func newTestingStream(buffer int) *testingStream {
	return &testingStream{
		incoming: make(chan []byte, buffer),
		outgoing: make(chan []byte, buffer),
		done:     make(chan struct{}),
	}
}

func (t *testingStream) Incoming() <-chan []byte {
	return t.incoming
}

func (t *testingStream) Outgoing() chan<- []byte {
	return t.outgoing
}

func (t *testingStream) Done() <-chan struct{} {
	return t.done
}
