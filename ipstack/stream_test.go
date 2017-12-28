package ipstack

import (
	"bytes"
	"testing"
	"time"
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

func TestMultiplexerPreFork(t *testing.T) {
	stream := newTestingStream(10)
	multi := Multiplex(stream, 10)
	defer multi.Close()

	stream.incoming <- []byte("hi")
	stream.incoming <- []byte("ih")

	time.Sleep(time.Second / 5)

	stream1, err := multi.Fork()
	if err != nil {
		t.Error(err)
	}
	for _, data := range [][]byte{[]byte("hi"), []byte("ih")} {
		if !bytes.Equal(<-stream1.Incoming(), data) {
			t.Error("unexpected packet")
		}
	}

	close(stream1.Outgoing())

	time.Sleep(time.Second / 5)
	stream.incoming <- []byte("foo")
	stream.incoming <- []byte("bar")
	time.Sleep(time.Second / 5)

	stream1, err = multi.Fork()
	if err != nil {
		t.Error(err)
	}
	for _, data := range [][]byte{[]byte("foo"), []byte("bar")} {
		if !bytes.Equal(<-stream1.Incoming(), data) {
			t.Error("unexpected packet")
		}
	}
}

// TODO: test closing parent MultiStream.

// TODO: test closing parent MultiStream while a write is
// blocked by backpressure.

// TODO: test closing child stream that is blocking the
// parent.

// TODO: test backpressure preventing writes to both
// parent and children.

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
