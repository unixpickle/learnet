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
	mgr := s.readDeadline.Wait()
	if mgr == nil {
		return nil, readTimeoutErr
	}
	defer s.readDeadline.Unwait(mgr)
	select {
	case <-mgr.Chan:
		return nil, readTimeoutErr
	case packet, ok := <-s.stream.Incoming():
		if !ok {
			return nil, errors.New("read: stream closed")
		}
		return packet, nil
	}
}

func (s *streamConn) WritePacket(b []byte) error {
	mgr := s.writeDeadline.Wait()
	if mgr == nil {
		return writeTimeoutErr
	}
	defer s.writeDeadline.Unwait(mgr)
	select {
	case <-mgr.Chan:
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

// A deadlineManager manages channels that are notified
// when a dynamically-changing deadline is exceeded.
type deadlineManager struct {
	lock          sync.Mutex
	exceeded      bool
	waiters       map[*deadlineWaiter]bool
	cancelCurrent chan<- struct{}
}

func newDeadlineManager() *deadlineManager {
	return &deadlineManager{waiters: map[*deadlineWaiter]bool{}}
}

// Wait creates a channel that waits for the deadline to
// be exceeded.
//
// Returns nil if the deadline was already exceeded.
// Otherwise, the caller must call Unwait() when it is
// done using the waiter.
func (d *deadlineManager) Wait() *deadlineWaiter {
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.exceeded {
		return nil
	}
	waiter := &deadlineWaiter{Chan: make(chan struct{})}
	d.waiters[waiter] = true
	return waiter
}

// Unwait cleans up a waiting channel from Wait().
func (d *deadlineManager) Unwait(w *deadlineWaiter) {
	d.lock.Lock()
	defer d.lock.Unlock()
	delete(d.waiters, w)
}

// SetDeadline updates the current deadline.
func (d *deadlineManager) SetDeadline(t time.Time) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.exceeded = false
	if d.cancelCurrent != nil {
		close(d.cancelCurrent)
		d.cancelCurrent = nil
	}
	if t.IsZero() {
		return
	}
	cancel := make(chan struct{})
	d.cancelCurrent = cancel
	go func() {
		select {
		case <-time.After(time.Until(t)):
		case <-cancel:
			return
		}

		d.lock.Lock()
		defer d.lock.Unlock()

		// Race condition to avoid announcing a deadline
		// after a new SetDeadline() has returned.
		select {
		case <-cancel:
			return
		default:
		}

		for waiter := range d.waiters {
			close(waiter.Chan)
		}
		d.waiters = map[*deadlineWaiter]bool{}
		d.exceeded = true
	}()
}

type deadlineWaiter struct {
	Chan chan struct{}
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
