package ipstack

import (
	"bytes"
	"sync"
	"testing"
	"time"
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

func TestMultiplexerClosePipeFlood(t *testing.T) {
	testMultiplexerCloseFlood(t, true)
}

func TestMultiplexerCloseMultiFlood(t *testing.T) {
	testMultiplexerCloseFlood(t, false)
}

func testMultiplexerCloseFlood(t *testing.T, closePipe bool) {
	parent, pipe := Pipe(10, 10)
	multi := Multiplex(parent)
	defer multi.Close()

	child, err := multi.Fork(10)
	if err != nil {
		t.Error(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				pipe.Write([]byte("hi"))
				select {
				case <-pipe.Done():
					return
				default:
				}
			}
		}()
	}

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

	for _ = range child.Incoming() {
	}
	<-child.Done()

	wg.Wait()
}
