package ipstack

import (
	"io"
	"sync"
	"time"
)

// A tcpSend manages the sending end of TCP.
type tcpSend interface {
	Write(b []byte) (int, error)
	Close() error

	// None of these should be called concurrently, except
	// with Write() and Close().
	Handle(ack uint32, window uint16)
	Fail(err error)
	Next() <-chan *tcpSegment
	Done() bool
}

type simpleTcpSend struct {
	maxSegmentSize uint16

	writeLock   sync.Mutex
	lock        sync.Mutex
	next        chan *tcpSegment
	notify      chan struct{}
	timerCancel chan struct{}
	failErr     error
	window      uint16
	writeBuf    tcpWriteBuffer
}

func newSimpleTcpSend(startSeq uint32, window, mss uint16) *simpleTcpSend {
	return &simpleTcpSend{
		maxSegmentSize: mss,
		next:           make(chan *tcpSegment, 16),
		notify:         make(chan struct{}),
		window:         window,
		writeBuf:       tcpWriteBuffer{sequence: startSeq},
	}
}

func (s *simpleTcpSend) Write(b []byte) (int, error) {
	return s.writeOrClose(b, false)
}

func (s *simpleTcpSend) Close() error {
	_, err := s.writeOrClose(nil, true)
	return err
}

func (s *simpleTcpSend) writeOrClose(data []byte, close bool) (int, error) {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	s.lock.Lock()
	if s.writeBuf.sentEOF {
		s.lock.Unlock()
		return 0, io.ErrClosedPipe
	} else if s.failErr != nil {
		s.lock.Unlock()
		return 0, s.failErr
	}
	s.writeBuf.SetData(data)
	if close {
		s.writeBuf.SetEOF()
	}
	notify := s.notify
	s.sendNext(false)
	s.lock.Unlock()
	<-notify

	s.lock.Lock()
	defer s.lock.Unlock()
	if s.failErr != nil {
		return len(data) - len(s.writeBuf.buffer), s.failErr
	}
	return len(data), nil
}

func (s *simpleTcpSend) Handle(ack uint32, window uint16) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.writeBuf.HandleAck(ack)
	s.cancelTimer()
	if s.writeBuf.BufferSize() == 0 {
		close(s.notify)
		s.notify = make(chan struct{})
	} else {
		s.sendNext(false)
	}

	s.window = window
	if s.window == 0 {
		s.startTimer()
	}
}

func (s *simpleTcpSend) Fail(err error) {
	s.lock.Lock()
	s.failErr = err
	close(s.notify)
	s.notify = make(chan struct{})
	s.lock.Unlock()
}

func (s *simpleTcpSend) Next() <-chan *tcpSegment {
	return s.next
}

func (s *simpleTcpSend) Done() bool {
	return s.writeBuf.sentEOF || s.failErr != nil
}

func (s *simpleTcpSend) sendNext(force bool) {
	windowSize := s.window
	if windowSize > s.maxSegmentSize {
		windowSize = s.maxSegmentSize
	}
	if payload := s.writeBuf.Next(windowSize, force); payload != nil {
		select {
		case s.next <- payload:
		default:
		}
		s.startTimer()
	}
}

func (s *simpleTcpSend) cancelTimer() {
	if s.timerCancel != nil {
		close(s.timerCancel)
		s.timerCancel = nil
	}
}

func (s *simpleTcpSend) startTimer() {
	s.cancelTimer()
	timerCancel := make(chan struct{})
	s.timerCancel = timerCancel
	go func() {
		time.Sleep(time.Second)
		s.lock.Lock()
		defer s.lock.Unlock()
		select {
		case <-timerCancel:
		default:
			s.timerCancel = nil
			s.sendNext(true)
		}
	}()
}

type tcpWriteBuffer struct {
	sequence uint32
	buffer   []byte
	hasEOF   bool
	sentEOF  bool
}

func (t *tcpWriteBuffer) SetData(data []byte) {
	t.buffer = data
}

func (t *tcpWriteBuffer) SetEOF() {
	t.hasEOF = true
}

func (t *tcpWriteBuffer) HandleAck(seq uint32) {
	offset := seq - t.sequence
	if offset > t.BufferSize() {
		return
	}
	if offset == t.BufferSize() {
		t.sequence += t.BufferSize()
		t.sentEOF = t.hasEOF
		t.buffer = nil
	} else {
		t.sequence += uint32(offset)
		t.buffer = t.buffer[offset:]
	}
}

func (t *tcpWriteBuffer) Next(window uint16, force bool) *tcpSegment {
	if t.BufferSize() == 0 {
		if force {
			return &tcpSegment{Start: t.sequence}
		} else {
			return nil
		}
	}
	if t.hasEOF && len(t.buffer) == 0 {
		return &tcpSegment{
			Start: t.sequence,
			Fin:   true,
		}
	}
	buffer := t.buffer
	if len(buffer) > int(window) {
		buffer = buffer[:window]
	}
	return &tcpSegment{
		Start: t.sequence,
		Data:  buffer,
	}
}

func (t *tcpWriteBuffer) BufferSize() uint32 {
	if t.hasEOF && !t.sentEOF {
		return uint32(len(t.buffer) + 1)
	} else {
		return uint32(len(t.buffer))
	}
}
