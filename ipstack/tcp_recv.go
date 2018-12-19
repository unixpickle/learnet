package ipstack

import (
	"io"
	"sync"
	"time"

	"github.com/unixpickle/essentials"
)

// A tcpRecv manages the receiving end of TCP.
type tcpRecv interface {
	// Read can be called from any Goroutine, and must block
	// until some data can be read or EOF is reached.
	Read(b []byte) (int, error)

	// SetDeadline can be called from any Goroutine.
	SetDeadline(t time.Time)

	// None of these methods should be called concurrently,
	// except with the above methods.
	Handle(segment *tcpSegment)
	Fail(err error)
	Ack() uint32
	Window() uint32
	Done() bool
}

type simpleTcpRecv struct {
	bufferMax int

	lock      sync.Mutex
	failErr   error
	assembler tcpAssembler
	notify    chan struct{}
	buffer    []byte
	deadline  *deadlineManager
}

func newSimpleTcpRecv(startSeq uint32, bufferMax int) *simpleTcpRecv {
	return &simpleTcpRecv{
		bufferMax: bufferMax,
		assembler: tcpAssembler{
			lastAcked: startSeq,
		},
		notify:   make(chan struct{}),
		deadline: newDeadlineManager(),
	}
}

func (s *simpleTcpRecv) Read(b []byte) (int, error) {
	select {
	case <-s.deadline.Chan():
		return 0, readTimeoutErr
	default:
	}

	s.lock.Lock()

	// If there's some data buffered, let's read it.
	if len(s.buffer) > 0 {
		if len(s.buffer) < len(b) {
			b = b[:len(s.buffer)]
		}
		copy(b, s.buffer)
		s.buffer = append([]byte{}, s.buffer[len(b):]...)
		s.lock.Unlock()
		return len(b), nil
	}

	if s.assembler.Finished() {
		s.lock.Unlock()
		return 0, io.EOF
	} else if s.failErr != nil {
		s.lock.Unlock()
		return 0, s.failErr
	}

	// Wait for data to arrive.
	notify := s.notify
	s.lock.Unlock()
	select {
	case <-notify:
	case <-s.deadline.Chan():
		return 0, readTimeoutErr
	}
	return s.Read(b)
}

func (s *simpleTcpRecv) SetDeadline(t time.Time) {
	s.deadline.SetDeadline(t)
}

func (s *simpleTcpRecv) Handle(segment *tcpSegment) {
	s.lock.Lock()
	newData := s.assembler.AddSegment(segment)
	s.buffer = append(s.buffer, newData...)
	close(s.notify)
	s.notify = make(chan struct{})
	s.lock.Unlock()
}

func (s *simpleTcpRecv) Fail(err error) {
	s.lock.Lock()
	s.failErr = err
	close(s.notify)
	s.notify = make(chan struct{})
	s.lock.Unlock()
}

func (s *simpleTcpRecv) Ack() uint32 {
	return s.assembler.Ack()
}

func (s *simpleTcpRecv) Window() uint32 {
	if len(s.buffer) > s.bufferMax {
		return 0
	}
	return uint32(s.bufferMax - len(s.buffer))
}

func (s *simpleTcpRecv) Done() bool {
	return s.assembler.Finished() || s.failErr != nil
}

type tcpAssembler struct {
	segments  []*tcpSegment
	lastAcked uint32
	finished  bool
}

func (t *tcpAssembler) AddSegment(s *tcpSegment) []byte {
	if t.finished || t.relStart(s)+int32(len(s.Data)) < 0 {
		return nil
	}

	t.segments = append(t.segments, s)
	essentials.VoodooSort(t.segments, func(i, j int) bool {
		return t.relStart(t.segments[i]) < t.relStart(t.segments[j])
	})

	// TODO: remove redundant segments.

	return t.skimFront()
}

func (t *tcpAssembler) skimFront() []byte {
	var res []byte
	var idx int32
	for i := 0; i < len(t.segments); i++ {
		seg := t.segments[i]
		start := t.relStart(seg)
		if start == idx {
			res = append(res, seg.Data...)
			idx += int32(len(seg.Data))
		} else if start < idx {
			if start+int32(len(seg.Data)) > idx {
				res = append(res, seg.Data[idx-start:]...)
				idx += int32(len(seg.Data)) - (idx - start)
			}
		} else {
			break
		}
		essentials.OrderedDelete(&t.segments, i)
		i--
		if seg.Fin {
			t.finished = true
			t.segments = nil
			idx++
			break
		}
	}
	t.lastAcked += uint32(idx)
	return res
}

func (t *tcpAssembler) Ack() uint32 {
	return t.lastAcked
}

func (t *tcpAssembler) Finished() bool {
	return t.finished
}

func (t *tcpAssembler) relStart(s *tcpSegment) int32 {
	return int32(s.Start - t.lastAcked)
}
