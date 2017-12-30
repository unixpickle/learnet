package ipstack

import (
	"bytes"
	"testing"
)

func TestMultiplexerBasic(t *testing.T) {
	parent, pipe := Pipe(10, 10)
	multi := Multiplex(parent)
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
	if !bytes.Equal(<-pipe.Incoming(), []byte("test")) {
		t.Error("unexpected packet")
	}

	stream2.Write([]byte("toast"))
	if !bytes.Equal(<-pipe.Incoming(), []byte("toast")) {
		t.Error("unexpected packet")
	}

	pipe.Write([]byte("foo"))
	pipe.Write([]byte("bar"))

	for _, child := range []Stream{stream1, stream2} {
		for _, data := range [][]byte{[]byte("foo"), []byte("bar")} {
			if !bytes.Equal(<-child.Incoming(), data) {
				t.Error("unexpected packet")
			}
		}
	}
}

func TestMultiplexerClose(t *testing.T) {
	parent, _ := Pipe(10, 10)
	multi := Multiplex(parent)
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
	parent, pipe := Pipe(10, 10)
	multi := Multiplex(parent)
	defer multi.Close()

	stream1, err := multi.Fork(10)
	if err != nil {
		t.Error(err)
	}

	stream2, err := multi.Fork(10)
	if err != nil {
		t.Error(err)
	}

	pipe.Close()

	<-stream1.Incoming()
	<-stream1.Done()

	<-stream2.Done()
	<-stream2.Incoming()
}

// TODO: test closing (both closing parent & multiplexer)
// when the stream is flooded with packets.
