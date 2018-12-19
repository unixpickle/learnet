package ipstack

import (
	"bytes"
	"errors"
	"testing"
)

func TestTCPAssembler(t *testing.T) {
	a := &tcpAssembler{
		lastAcked: 0xfffffffe,
	}
	res := a.AddSegment(&tcpSegment{
		Start: 10,
		Data:  []byte("hi!"),
		Fin:   true,
	})
	if len(res) != 0 || a.finished {
		t.Fatal("unexpected result")
	}
	res = a.AddSegment(&tcpSegment{
		Start: 1,
		Data:  []byte("eyy"),
		Fin:   false,
	})
	if len(res) != 0 || a.finished {
		t.Fatal("unexpected result")
	}
	res = a.AddSegment(&tcpSegment{
		Start: 0xfffffffe,
		Data:  []byte("hhhey"),
		Fin:   false,
	})
	if !bytes.Equal(res, []byte("hhheyy")) {
		t.Fatal("unexpected bytes")
	}
	if a.finished || a.lastAcked != 4 {
		t.Fatal("unexpected state")
	}
	res = a.AddSegment(&tcpSegment{
		Start: 4,
		Data:  []byte("hello!"),
		Fin:   false,
	})
	if !bytes.Equal(res, []byte("hello!hi!")) {
		t.Fatal("unexpected bytes")
	}
	if !a.finished || a.lastAcked != 14 {
		t.Fatal("unexpected state")
	}
}

func TestTCPRecvFail(t *testing.T) {
	done := make(chan struct{})
	recv := newSimpleTcpRecv(1337, 1000)
	go func() {
		data := make([]byte, 100)
		_, err := recv.Read(data)
		if err == nil || err.Error() != "error!" {
			t.Fatal("invalid error", err)
		}
		close(done)
	}()
	recv.Fail(errors.New("error!"))
	<-done
}

func TestTCPRecvSuccess(t *testing.T) {
	done := make(chan struct{})
	recv := newSimpleTcpRecv(1337, 1000)
	go func() {
		data := make([]byte, 100)
		n, err := recv.Read(data)
		if err != nil {
			t.Fatal(err)
		}
		if n != 3 || !bytes.Equal(data[:3], []byte("hi!")) {
			t.Fatal("invalid results")
		}
		close(done)
	}()
	recv.Handle(&tcpSegment{
		Start: 1338,
		Data:  []byte("i!"),
		Fin:   true,
	})
	if recv.Ack() != 1337 || recv.Done() {
		t.Fatal("invalid state")
	}
	recv.Handle(&tcpSegment{
		Start: 1337,
		Data:  []byte("h"),
	})
	if recv.Ack() != 1341 || !recv.Done() {
		t.Fatal("invalid state")
	}
	<-done
}
