package ipstack

import (
	"io"
	"sync"
	"time"
)

// A tcpRecv manages the receiving end of TCP.
//
// The Read and SetDeadline methods may be called from any
// Goroutine. All other methods should only be called one
// at a time.
type tcpRecv interface {
	// Read blocks until some data can be read, EOF is
	// reached, or an error occurs due to Fail().
	Read(b []byte) (int, error)

	// SetDeadline updates the deadline for all reads.
	SetDeadline(t time.Time)

	// Handle notifies the receiver of incoming data.
	// This may cause reads to unblock.
	Handle(segment *tcpSegment)

	// Fail notifies the receiver of some kind of
	// connection error.
	// Errors are processed after all buffered data has
	// been read.
	// This may cause reads to unblock.
	Fail(err error)

	// Ack gets the first unacknowledged sequence number.
	Ack() uint32

	// Window gets the current window size.
	Window() uint16

	// WindowOpen is a channel which is sent a value when
	// the window size goes from zero to non-zero.
	WindowOpen() <-chan struct{}

	// Done checks if the receiver no longer needs any
	// more packets to finish up.
	// This will only change due to Fail() or Handle().
	Done() bool
}

type simpleTcpRecv struct {
	lock       sync.Mutex
	failErr    error
	assembler  *tcpAssembler
	buffer     *tcpRecvBuffer
	notify     chan struct{}
	windowOpen chan struct{}
	deadline   *deadlineManager
}

func newSimpleTcpRecv(startSeq uint32, bufSize int) *simpleTcpRecv {
	return &simpleTcpRecv{
		assembler:  newTCPAssembler(startSeq),
		buffer:     newTCPRecvBuffer(bufSize),
		notify:     make(chan struct{}),
		windowOpen: make(chan struct{}, 1),
		deadline:   newDeadlineManager(),
	}
}

func (s *simpleTcpRecv) Read(b []byte) (int, error) {
	select {
	case <-s.deadline.Chan():
		return 0, readTimeoutErr
	default:
	}

	s.lock.Lock()

	oldWindow := s.buffer.Window()
	numBytes, eof := s.buffer.Get(b)
	if s.buffer.Window() != 0 && oldWindow == 0 {
		select {
		case s.windowOpen <- struct{}{}:
		default:
		}
	}

	if numBytes > 0 || eof {
		s.lock.Unlock()
		if eof {
			return numBytes, io.EOF
		} else {
			return numBytes, nil
		}
	}

	// Only yield an error once the read buffer is
	// completed.
	if s.failErr != nil {
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
	s.assembler.AddSegment(segment)
	newData, eof := s.assembler.Skim(s.buffer.Window())
	s.buffer.Put(newData)
	if eof {
		s.buffer.PutEOF()
	}
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
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.assembler.Seq()
}

func (s *simpleTcpRecv) Window() uint16 {
	s.lock.Lock()
	defer s.lock.Unlock()
	return uint16(s.buffer.Window())
}

func (s *simpleTcpRecv) WindowOpen() <-chan struct{} {
	return s.windowOpen
}

func (s *simpleTcpRecv) Done() bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.buffer.hitEOF || s.failErr != nil
}

// A tcpAssembler is used to assemble received segments of
// data into contiguous chunks.
type tcpAssembler struct {
	// The sequence number of the start of the buffer.
	sequence uint32

	// Data in the pipeline.
	buffer []byte

	// A boolean for each byte of buffer indicating if
	// that byte has been received.
	mask []bool

	// If greater than -2, indicates the position in the
	// buffer that represents EOF. This should come after
	// the index of the final byte. May be len(buffer).
	// May be -1 to indicate that EOF has been seen.
	fin int
}

func newTCPAssembler(seq uint32) *tcpAssembler {
	return &tcpAssembler{
		sequence: seq,
		buffer:   make([]byte, 65536),
		mask:     make([]bool, 65536),
		fin:      -2,
	}
}

// AddSegment tells the assembler about new incoming data.
func (t *tcpAssembler) AddSegment(s *tcpSegment) {
	if t.fin == 0 || t.fin == -1 {
		return
	}
	for i, b := range s.Data {
		offset := uint32(i) + s.Start - t.sequence
		if offset >= uint32(len(t.buffer)) {
			continue
		}
		t.buffer[offset] = b
		t.mask[offset] = true
	}
	if s.Fin {
		finOffset := s.Start + uint32(len(s.Data)) - t.sequence
		if finOffset <= uint32(len(t.buffer)) {
			t.fin = int(finOffset)
		}
	}
}

// Skim gets the available data from the front of the
// buffer and potentially signals EOF.
func (t *tcpAssembler) Skim(maxBytes int) (avail []byte, eof bool) {
	if t.fin == -1 {
		return nil, true
	}
	for i, f := range t.mask {
		if i == t.fin {
			eof = true
			break
		}
		if i == maxBytes || !f {
			break
		}
		avail = append(avail, t.buffer[i])
	}
	readSize := len(avail)
	if eof {
		readSize += 1
	}
	t.sequence += uint32(readSize)
	t.fin -= readSize
	copy(t.buffer, t.buffer[readSize:])
	copy(t.mask, t.mask[readSize:])
	for i := len(t.mask) - readSize; i < len(t.mask); i++ {
		t.mask[i] = false
	}
	return
}

// Seq gets the first unread sequence number.
func (t *tcpAssembler) Seq() uint32 {
	return t.sequence
}

// A tcpRecvBuffer performs flow control for incoming TCP
// data.
type tcpRecvBuffer struct {
	// The number of readable bytes in the buffer.
	size int

	// The buffer of available bytes.
	buffer []byte

	// A flag which is set to true if EOF should be
	// produced once the buffer is drained.
	hitEOF bool
}

func newTCPRecvBuffer(bufSize int) *tcpRecvBuffer {
	return &tcpRecvBuffer{
		size:   0,
		buffer: make([]byte, bufSize),
	}
}

// Put adds data to the buffer.
// The data must fit into the buffer.
func (t *tcpRecvBuffer) Put(data []byte) {
	if len(data) > t.Window() {
		panic("buffer overflow")
	}
	copy(t.buffer[t.size:], data)
	t.size += len(data)
}

// PutEOF adds an EOF to the end of the buffer.
func (t *tcpRecvBuffer) PutEOF() {
	t.hitEOF = true
}

// Get reads up to len(b) bytes from the buffer and
// returns the number of bytes read, as well as an EOF
// flag.
func (t *tcpRecvBuffer) Get(b []byte) (int, bool) {
	canRead := t.size
	if canRead > len(b) {
		canRead = len(b)
	}
	copy(b, t.buffer[:canRead])
	copy(t.buffer, t.buffer[canRead:])
	t.size -= canRead
	return canRead, t.size == 0 && t.hitEOF
}

// Window gets the number of unused bytes in the buffer.
func (t *tcpRecvBuffer) Window() int {
	return len(t.buffer) - t.size
}
