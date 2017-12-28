package ipstack

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

func TestMultiplexerBasic(t *testing.T) {
	stream := newTestingStream(10)
	multi := Multiplex(stream, 10, true)
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
	multi := Multiplex(stream, 10, true)
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

func TestMultiplexerClose(t *testing.T) {
	stream := newTestingStream(10)
	multi := Multiplex(stream, 10, true)
	defer multi.Close()

	stream1, err := multi.Fork()
	if err != nil {
		t.Error(err)
	}

	stream2, err := multi.Fork()
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
	multi := Multiplex(stream, 10, true)
	defer multi.Close()

	stream1, err := multi.Fork()
	if err != nil {
		t.Error(err)
	}

	stream2, err := multi.Fork()
	if err != nil {
		t.Error(err)
	}

	close(stream.incoming)
	close(stream.done)

	<-stream1.Incoming()
	<-stream1.Done()

	<-stream2.Done()
	<-stream2.Incoming()
}

func TestMultiplexerCloseBackpressure(t *testing.T) {
	stream := newTestingStream(10)
	multi := Multiplex(stream, 10, true)
	defer multi.Close()

	child, err := multi.Fork()
	if err != nil {
		t.Error(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for {
			select {
			case stream.incoming <- []byte("hi1"):
			case <-child.Done():
				wg.Done()
				return
			}
		}
	}()

	// Make sure messages are flooding in.
	for i := 0; i < 100; i++ {
		<-child.Incoming()
	}

	time.Sleep(time.Second / 5)
	multi.Close()

	for _ = range child.Incoming() {
	}
	<-child.Done()

	wg.Wait()
}

func TestMultiplexerCloseChildBackpressure(t *testing.T) {
	stream := newTestingStream(10)
	multi := Multiplex(stream, 10, true)
	defer multi.Close()

	child, err := multi.Fork()
	if err != nil {
		t.Error(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for {
			select {
			case stream.incoming <- []byte("hi1"):
			case <-child.Done():
				wg.Done()
				return
			}
		}
	}()

	time.Sleep(time.Second / 5)
	close(child.Outgoing())

	child, err = multi.Fork()
	if err != nil {
		t.Error(err)
	}

	wg.Add(1)
	go func() {
		for {
			select {
			case stream.incoming <- []byte("hi2"):
			case <-child.Done():
				wg.Done()
				return
			}
		}
	}()

	for packet := range child.Incoming() {
		if bytes.Equal(packet, []byte("hi2")) {
			break
		}
	}
	close(child.Outgoing())

	wg.Wait()
}

func TestMultiplexerReadBackpressure(t *testing.T) {
	stream := newTestingStream(10)
	multi := Multiplex(stream, 10, true)
	defer multi.Close()

	child, err := multi.Fork()
	if err != nil {
		t.Error(err)
	}

	for i := 0; i < 22; i++ {
		stream.incoming <- []byte("foo")
	}

	time.Sleep(time.Second / 5)
	select {
	case stream.incoming <- []byte("bar"):
		t.Error("expected backpressure")
	default:
	}

	<-child.Incoming()

	time.Sleep(time.Second / 5)
	select {
	case stream.incoming <- []byte("bar"):
	default:
		t.Error("expected no backpressure")
	}
}

func TestMultiplexerWriteBackpressure(t *testing.T) {
	stream := newTestingStream(10)
	multi := Multiplex(stream, 10, true)
	defer multi.Close()

	child, err := multi.Fork()
	if err != nil {
		t.Error(err)
	}

	for i := 0; i < 22; i++ {
		child.Outgoing() <- []byte("foo")
	}

	time.Sleep(time.Second / 5)
	select {
	case child.Outgoing() <- []byte("bar"):
		t.Error("expected backpressure")
	default:
	}

	<-stream.outgoing

	time.Sleep(time.Second / 5)
	select {
	case child.Outgoing() <- []byte("bar"):
	default:
		t.Error("expected no backpressure")
	}
}

func TestMultiplexerDropOnEmpty(t *testing.T) {
	stream := newTestingStream(10)
	multi := Multiplex(stream, 10, false)
	defer multi.Close()

	stream.incoming <- []byte("hi")

	time.Sleep(time.Second / 5)

	child, err := multi.Fork()
	if err != nil {
		t.Error(err)
	}

	time.Sleep(time.Second / 5)

	select {
	case <-child.Incoming():
		t.Error("expected no pending packets")
	default:
	}
}

func TestMultiplexerBlockOnEmpty(t *testing.T) {
	stream := newTestingStream(10)
	multi := Multiplex(stream, 10, true)
	defer multi.Close()

	stream.incoming <- []byte("hi")

	time.Sleep(time.Second / 5)

	child, err := multi.Fork()
	if err != nil {
		t.Error(err)
	}

	time.Sleep(time.Second / 5)

	select {
	case buffer := <-child.Incoming():
		if !bytes.Equal(buffer, []byte("hi")) {
			t.Error("bad buffer")
		}
	default:
		t.Error("expected a pending packet")
	}
}

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
