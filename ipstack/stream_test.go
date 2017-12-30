package ipstack

import (
	"bytes"
	"testing"
)

func TestMultiplexerBasic(t *testing.T) {
	stream := newTestingStream(10)
	multi := Multiplex(stream)
	defer multi.Close()

	stream1, err := multi.Fork(10)
	if err != nil {
		t.Error(err)
	}
	stream2, err := multi.Fork(10)
	if err != nil {
		t.Error(err)
	}

	stream1.Write([]byte("test"))
	if !bytes.Equal(<-stream.outgoing, []byte("test")) {
		t.Error("unexpected packet")
	}

	stream2.Write([]byte("toast"))
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

func TestMultiplexerClose(t *testing.T) {
	stream := newTestingStream(10)
	multi := Multiplex(stream)
	defer multi.Close()

	stream1, err := multi.Fork(10)
	if err != nil {
		t.Error(err)
	}

	stream2, err := multi.Fork(10)
	if err != nil {
		t.Error(err)
	}

	multi.Close()

	<-stream1.Incoming()
	<-stream1.Done()

	<-stream2.Done()
	<-stream2.Incoming()
}

func TestMultiplexerParentClose(t *testing.T) {
	stream := newTestingStream(10)
	multi := Multiplex(stream)
	defer multi.Close()

	stream1, err := multi.Fork(10)
	if err != nil {
		t.Error(err)
	}

	stream2, err := multi.Fork(10)
	if err != nil {
		t.Error(err)
	}

	stream.Close()

	<-stream1.Incoming()
	<-stream1.Done()

	<-stream2.Done()
	<-stream2.Incoming()
}

// TODO: test closing (both closing parent & multiplexer)
// when the stream is flooded with packets.

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

func (t *testingStream) Write(buf []byte) error {
	select {
	case t.outgoing <- buf:
		return nil
	default:
		return WriteBufferFullErr
	}
}

func (t *testingStream) Close() error {
	close(t.incoming)
	close(t.done)
	return nil
}

func (t *testingStream) Done() <-chan struct{} {
	return t.done
}
