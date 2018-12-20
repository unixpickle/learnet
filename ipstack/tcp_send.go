package ipstack

import (
	"io"
	"sync"
	"time"
)

// A tcpSend manages the sending end of TCP.
//
// Write(), Close(), and SetDeadline() may be called from
// any Goroutine. All other methods should only be called
// one at a time.
type tcpSend interface {
	// Write writes all of the data to the connection, or
	// yields an error caused by Fail().
	Write(b []byte) (int, error)

	// Close triggers an EOF sequence.
	Close() error

	// SetDeadline sets the deadline for all writes.
	SetDeadline(t time.Time)

	// Handle updates the sender's state based on the ack
	// and window size. This may trigger new packets to be
	// sent to Next().
	Handle(ack uint32, window uint16)

	// Fail triggers an error for all subsequent writes.
	Fail(err error)

	// Next is a channel of desired outgoing segments.
	// Not reading this will cause segments to be dropped.
	Next() <-chan *tcpSegment

	// Seq gets the first sequence number not sent.
	Seq() uint32

	// Done checks if the sender has no more segments to
	// send.
	Done() bool
}

type simpleTcpSend struct {
	maxSegmentSize uint16

	writeLock sync.Mutex
	lock      sync.Mutex
	notify    chan struct{}
	writeBuf  *tcpWriteBuffer
	timer     *tcpSendTimer
	failErr   error
	window    uint16
	deadline  *deadlineManager
}

func newSimpleTcpSend(startSeq uint32, window, mss uint16) *simpleTcpSend {
	return &simpleTcpSend{
		maxSegmentSize: mss,
		notify:         make(chan struct{}),
		writeBuf:       newTCPWriteBuffer(startSeq),
		timer:          newTcpSendTimer(),
		window:         window,
		deadline:       newDeadlineManager(),
	}
}

func (s *simpleTcpSend) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	return s.writeOrClose(b, false)
}

func (s *simpleTcpSend) Close() error {
	_, err := s.writeOrClose(nil, true)
	return err
}

func (s *simpleTcpSend) writeOrClose(data []byte, close bool) (int, error) {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	if !close {
		select {
		case <-s.deadline.Chan():
			return 0, writeTimeoutErr
		default:
		}
	}

	s.lock.Lock()

	if s.writeBuf.sendEOF {
		s.lock.Unlock()
		return 0, io.ErrClosedPipe
	} else if s.failErr != nil {
		s.lock.Unlock()
		return 0, s.failErr
	}

	if close {
		s.writeBuf.SetEOF()
	} else {
		s.writeBuf.SetData(data)
	}
	s.sendNext()

	notify := s.notify
	s.lock.Unlock()

	select {
	case <-notify:
	case <-s.deadline.Chan():
		return 0, writeTimeoutErr
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	if s.failErr != nil {
		return len(data) - len(s.writeBuf.buffer), s.failErr
	}
	return len(data), nil
}

func (s *simpleTcpSend) SetDeadline(t time.Time) {
	s.deadline.SetDeadline(t)
}

func (s *simpleTcpSend) Handle(ack uint32, window uint16) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.writeBuf.Handle(ack)
	s.window = window
	s.sendNext()
	if s.writeBuf.Remaining() == 0 {
		close(s.notify)
		s.notify = make(chan struct{})
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
	return s.timer.Chan()
}

func (s *simpleTcpSend) Seq() uint32 {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.writeBuf.sequence
}

func (s *simpleTcpSend) Done() bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.writeBuf.sentEOF || s.failErr != nil
}

func (s *simpleTcpSend) sendNext() {
	s.timer.Cancel()

	if s.writeBuf.Remaining() == 0 {
		return
	}

	if s.window == 0 {
		// "Persist timer" to prevent deadlock.
		s.timer.Schedule(s.writeBuf.Segment(1))
		return
	}

	max := s.window
	if max > s.maxSegmentSize {
		max = s.maxSegmentSize
	}
	s.timer.Send(s.writeBuf.Segment(max))
}

// A tcpWriteBuffer maintains information about the
// current outgoing chunk of data for a simpleTcpsend.
// The outgoing buffer can either be a chunk of data
// passed to Write(), or a single EOF (FIN) signal.
type tcpWriteBuffer struct {
	// The sequence number of the start of the buffer.
	// If sendEOF is true, then instead of the buffer,
	// this corresponds to the start of the EOF, or the
	// end of the EOF if sentEOF is also true.
	sequence uint32

	// If EOF is being sent, then sendEOF is true.
	// At that point, buffer must be empty.
	sendEOF bool

	// If EOF has been acknowledged, then sentEOF is true.
	sentEOF bool

	// The data to be sent.
	buffer []byte
}

func newTCPWriteBuffer(seq uint32) *tcpWriteBuffer {
	return &tcpWriteBuffer{sequence: seq}
}

// SetData sets the outgoing data to send.
func (t *tcpWriteBuffer) SetData(buffer []byte) {
	if len(t.buffer) > 0 || t.sendEOF {
		panic("already sending something")
	}
	t.buffer = buffer
}

// SetEOF sets the outgoing data to an EOF.
func (t *tcpWriteBuffer) SetEOF() {
	if len(t.buffer) > 0 {
		panic("already sending something")
	}
	t.sendEOF = true
}

// Handle updates the buffer based on an acknowledgement.
func (t *tcpWriteBuffer) Handle(ack uint32) {
	offset := ack - t.sequence

	// This is really a "less than" in circular arithmetic.
	if offset > t.Remaining() {
		return
	}

	if offset == t.Remaining() {
		t.sequence += t.Remaining()
		t.sentEOF = t.sendEOF
		t.buffer = nil
	} else {
		t.sequence += uint32(offset)
		t.buffer = t.buffer[offset:]
	}
}

// Segment creates the next tcpSegment.
// The segment will not be larger than maxSize bytes.
//
// This should only be called if t.Remaining() > 0.
func (t *tcpWriteBuffer) Segment(maxSize uint16) *tcpSegment {
	if t.Remaining() == 0 {
		panic("no data to write")
	}

	if t.sendEOF {
		return &tcpSegment{
			Start: t.sequence,
			Fin:   true,
		}
	}

	buffer := t.buffer
	if len(buffer) > int(maxSize) {
		buffer = buffer[:maxSize]
	}
	return &tcpSegment{
		Start: t.sequence,
		Data:  buffer,
	}
}

// Remaining gets the number of sequence increments that
// still must take place to finish the buffer.
func (t *tcpWriteBuffer) Remaining() uint32 {
	if t.sendEOF && !t.sentEOF {
		return 1
	}
	return uint32(len(t.buffer))
}

// A tcpSendTimer manages retransmission and persist
// timers for a simpleTcpSend.
type tcpSendTimer struct {
	lock  sync.Mutex
	next  chan *tcpSegment
	timer *time.Timer
}

func newTcpSendTimer() *tcpSendTimer {
	return &tcpSendTimer{
		next: make(chan *tcpSegment, 1),
	}
}

// Chan gets the channel of outgoing segments.
func (t *tcpSendTimer) Chan() <-chan *tcpSegment {
	return t.next
}

// Cancel stops any retry or persist timers.
func (t *tcpSendTimer) Cancel() {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
}

// Schedule schedules a segment to be transmitted after a
// reasonable time interval.
//
// This should be used for persist timers.
func (t *tcpSendTimer) Schedule(seg *tcpSegment) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.schedule(seg)
}

func (t *tcpSendTimer) schedule(seg *tcpSegment) {
	if t.timer != nil {
		t.timer.Stop()
	}
	t.timer = time.AfterFunc(time.Second, func() {
		t.Send(seg)
	})
}

// Send sends a segment to the next channel, and schedules
// the segment to be resent if no Cancel() is called in a
// reasonable time interval.
func (t *tcpSendTimer) Send(seg *tcpSegment) {
	t.lock.Lock()
	defer t.lock.Unlock()
	select {
	case t.next <- seg:
	default:
	}
	t.schedule(seg)
}
