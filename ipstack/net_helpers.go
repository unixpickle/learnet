package ipstack

import (
	"errors"
	"sync"
	"time"
)

var (
	readTimeoutErr  = &timeoutError{Context: "read"}
	writeTimeoutErr = &timeoutError{Context: "write"}
)

// streamConn is a helper for making net.Conns that
// transfer packets on a Stream.
type streamConn struct {
	stream        Stream
	readDeadline  *deadlineManager
	writeDeadline *deadlineManager
}

func newStreamConn(stream Stream) *streamConn {
	return &streamConn{
		stream:        stream,
		readDeadline:  newDeadlineManager(),
		writeDeadline: newDeadlineManager(),
	}
}

func (s *streamConn) ReadPacket() ([]byte, error) {
	deadline := s.readDeadline.Chan()
	select {
	case <-deadline:
		return nil, readTimeoutErr
	default:
	}
	select {
	case <-deadline:
		return nil, readTimeoutErr
	case packet, ok := <-s.stream.Incoming():
		if !ok {
			return nil, errors.New("read: stream closed")
		}
		return packet, nil
	}
}

func (s *streamConn) WritePacket(b []byte) error {
	deadline := s.writeDeadline.Chan()
	select {
	case <-deadline:
		return writeTimeoutErr
	default:
	}
	select {
	case <-deadline:
		return writeTimeoutErr
	case s.stream.Outgoing() <- b:
		return nil
	case <-s.stream.Done():
		return errors.New("write: stream closed")
	}
}

func (s *streamConn) Close() error {
	return s.stream.Close()
}

func (s *streamConn) SetDeadline(t time.Time) error {
	s.SetReadDeadline(t)
	s.SetWriteDeadline(t)
	return nil
}

func (s *streamConn) SetReadDeadline(t time.Time) error {
	s.readDeadline.SetDeadline(t)
	return nil
}

func (s *streamConn) SetWriteDeadline(t time.Time) error {
	s.writeDeadline.SetDeadline(t)
	return nil
}

type deadlineManager struct {
	lock     sync.Mutex
	curTimer *time.Timer
	curChan  chan struct{}
}

func newDeadlineManager() *deadlineManager {
	return &deadlineManager{curChan: make(chan struct{})}
}

func (d *deadlineManager) Chan() <-chan struct{} {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.curChan
}

func (d *deadlineManager) SetDeadline(deadline time.Time) {
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.curTimer != nil {
		d.curTimer.Stop()
		d.curTimer = nil
	}
	select {
	case <-d.curChan:
		d.curChan = make(chan struct{})
	default:
	}
	if !deadline.IsZero() {
		d.curTimer = time.AfterFunc(deadline.Sub(time.Now()), func() {
			d.lock.Lock()
			defer d.lock.Unlock()
			select {
			case <-d.curChan:
			default:
				close(d.curChan)
			}
		})
	}
}

type timeoutError struct {
	Context string
}

func (t *timeoutError) Error() string {
	return t.Context + ": operation timeout"
}

func (t *timeoutError) Timeout() bool {
	return true
}
