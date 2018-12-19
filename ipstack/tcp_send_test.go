package ipstack

import (
	"bytes"
	"errors"
	"testing"
)

func TestTCPSendNormal(t *testing.T) {
	sender := newSimpleTcpSend(1337, 1000, 512)
	done := make(chan struct{})
	go func() {
		n, err := sender.Write([]byte("hello, world!"))
		if n != 13 || err != nil {
			t.Fatal("unexpected result:", n, err)
		}
		err = sender.Close()
		if err != nil {
			t.Fatal("close error:", err)
		}
		close(done)
	}()
	seg1 := <-sender.Next()
	if seg1.Start != 1337 || !bytes.Equal(seg1.Data, []byte("hello, world!")) || seg1.Fin {
		t.Fatal("unexpected segment")
	}
	sender.Handle(seg1.Start+uint32(len(seg1.Data)), 1000)
	seg2 := <-sender.Next()
	if seg2.Start != 1337+13 || len(seg2.Data) != 0 || !seg2.Fin {
		t.Fatal("unexpected segment")
	}
	if sender.Done() {
		t.Fatal("done before final ack")
	}
	sender.Handle(1337+14, 1000)
	if !sender.Done() {
		t.Fatal("not done after final ack")
	}
	<-done
}

func TestTCPSendFail(t *testing.T) {
	sender := newSimpleTcpSend(1337, 1000, 512)
	done := make(chan struct{})
	go func() {
		_, err := sender.Write([]byte("hello, world!"))
		if err == nil || err.Error() != "error!" {
			t.Fatal("unexpected error")
		}
		close(done)
	}()
	sender.Fail(errors.New("error!"))
	<-done
}
