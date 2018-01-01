package ipstack

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

func TestMultiplexerBasic(t *testing.T) {
	parent, pipe := Pipe(10)
	multi := Multiplex(parent)
	defer multi.Close()

	stream1, err := multi.Fork(10)
	if err != nil {
		t.Fatal(err)
	}
	stream2, err := multi.Fork(10)
	if err != nil {
		t.Fatal(err)
	}

	Send(stream1, []byte("test"))
	if !bytes.Equal(<-pipe.Incoming(), []byte("test")) {
		t.Error("unexpected packet")
	}

	Send(stream1, []byte("toast"))
	if !bytes.Equal(<-pipe.Incoming(), []byte("toast")) {
		t.Error("unexpected packet")
	}

	Send(pipe, []byte("foo"))
	Send(pipe, []byte("bar"))

	for _, child := range []Stream{stream1, stream2} {
		for _, data := range [][]byte{[]byte("foo"), []byte("bar")} {
			if !bytes.Equal(<-child.Incoming(), data) {
				t.Error("unexpected packet")
			}
		}
	}
}

func TestMultiplexerClose(t *testing.T) {
	parent, _ := Pipe(10)
	multi := Multiplex(parent)
	defer multi.Close()

	stream1, err := multi.Fork(10)
	if err != nil {
		t.Fatal(err)
	}

	stream2, err := multi.Fork(10)
	if err != nil {
		t.Fatal(err)
	}

	multi.Close()

	<-stream1.Incoming()
	<-stream1.Done()

	<-stream2.Done()
	<-stream2.Incoming()
}

func TestMultiplexerParentClose(t *testing.T) {
	parent, pipe := Pipe(10)
	multi := Multiplex(parent)
	defer multi.Close()

	stream1, err := multi.Fork(10)
	if err != nil {
		t.Fatal(err)
	}

	stream2, err := multi.Fork(10)
	if err != nil {
		t.Fatal(err)
	}

	pipe.Close()

	<-stream1.Incoming()
	<-stream1.Done()

	<-stream2.Done()
	<-stream2.Incoming()
}

func TestMultiplexerWritePressure(t *testing.T) {
	parent, pipe := Pipe(10)
	multi := Multiplex(parent)
	defer multi.Close()

	child, err := multi.Fork(10)
	if err != nil {
		t.Fatal(err)
	}

	// Each child stream can buffer one packet.
	// The pipe channel can buffer 10 packets, plus 2 that it
	// can store in memory as it forwards between channels.

	for i := 0; i < 13; i++ {
		child.Outgoing() <- []byte("in a buffer")
	}

	select {
	case child.Outgoing() <- []byte("cannot buffer"):
		t.Error("backpressure expected")
	case <-time.After(time.Second / 5):
	}

	<-pipe.Incoming()

	select {
	case child.Outgoing() <- []byte("foo"):
	case <-time.After(time.Second / 5):
		t.Error("backpressure not expected")
	}
}

func TestMultiplexerClosePipeFlood(t *testing.T) {
	testMultiplexerCloseFlood(t, true)
}

func TestMultiplexerCloseMultiFlood(t *testing.T) {
	testMultiplexerCloseFlood(t, false)
}

func testMultiplexerCloseFlood(t *testing.T, closePipe bool) {
	parent, pipe := Pipe(10)
	multi := Multiplex(parent)
	defer multi.Close()

	child, err := multi.Fork(10)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if Send(pipe, []byte("hi")) != nil {
					return
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if Send(child, []byte("hello!")) != nil {
				return
			}
		}
	}()

	// Make sure messages are flooding in.
	for i := 0; i < 100; i++ {
		<-child.Incoming()
	}

	time.Sleep(time.Second / 5)

	if closePipe {
		pipe.Close()
	} else {
		multi.Close()
	}

	<-child.Done()
	for _ = range child.Incoming() {
	}

	wg.Wait()
}
