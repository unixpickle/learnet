package ipstack

import (
	"errors"
	"math/rand"
	"time"
)

const tcpNumRetries = 10

type tcpHandshake struct {
	localSeq  uint32
	remoteSeq uint32

	localWinSize  uint16
	remoteWinSize uint16

	mss uint16
}

// tcp4ServerHandshake performs the handshake from the
// server side.
func tcp4ServerHandshake(stream Stream, syn TCP4Packet, ttl int) (*tcpHandshake, error) {
	localSeq := rand.Uint32()
	synAck := NewTCP4Packet(ttl, syn.DestAddr(), syn.SourceAddr(), localSeq,
		syn.Header().SeqNum()+1, 1000, nil, SYN, ACK)
OuterLoop:
	for i := 0; i < tcpNumRetries; i++ {
		select {
		case <-stream.Done():
			return nil, errors.New("stream closed")
		default:
		}
		select {
		case stream.Outgoing() <- synAck:
		default:
		}
		timeout := time.After(time.Second)
		for {
			select {
			case <-timeout:
				continue OuterLoop
			case packet := <-stream.Incoming():
				if packet == nil {
					return nil, errors.New("stream closed")
				}
				tp := TCP4Packet(packet)
				if tp.Header().Flag(ACK) && !tp.Header().Flag(SYN) &&
					tp.Header().AckNum() == localSeq+1 {
					return &tcpHandshake{
						localSeq:      localSeq + 1,
						remoteSeq:     syn.Header().SeqNum() + 1,
						localWinSize:  1000,
						remoteWinSize: tp.Header().WindowSize(),
						// TODO: read MSS from options.
						mss: 128,
					}, nil
				}
			}
		}
	}
	return nil, errors.New("connection failed")
}
