package ipstack

import (
	"bytes"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/unixpickle/essentials"
)

func TestFragmentation(t *testing.T) {
	sender, receiver := Pipe(0)
	sender = newRandomLatencyStream(sender)
	sender = AddIPv4Identifiers(FragmentOutgoingIPv4(sender, 133))
	receiver = FilterIPv4Valid(receiver)
	receiver = FilterIPv4Checksums(receiver)
	receiver = DefragmentIncomingIPv4(receiver, time.Second*3)

	packets := make([][]byte, 30)
	for i := range packets {
		payload := make([]byte, rand.Intn(300)+30)
		for j := 0; j < len(payload); j++ {
			payload[j] = byte(rand.Intn(0x100))
		}
		packets[i] = NewIPv4Packet(rand.Intn(30)+1, ProtocolNumberICMP, net.IP{10, 0, 0, 1},
			net.IP{10, 0, 0, 2}, payload)
		if err := Send(sender, packets[i]); err != nil {
			t.Fatal(err)
		}
	}

	timeout := time.After(time.Second * 5)

IncomingLoop:
	for len(packets) > 0 {
		select {
		case <-timeout:
			t.Fatal("got timeout with", len(packets), "packets remaining")
		case packet := <-receiver.Incoming():
			for i, other := range packets {
				if bytes.Equal(packet, other) {
					essentials.UnorderedDelete(&packets, i)
					continue IncomingLoop
				}
			}
			t.Error("got unrecognized packet")
		}
	}
}

type randomLatencyStream struct {
	Stream
	outgoing chan []byte
}

func newRandomLatencyStream(s Stream) Stream {
	res := &randomLatencyStream{Stream: s, outgoing: make(chan []byte)}
	go res.forwardLoop()
	return res
}

func (r *randomLatencyStream) Outgoing() chan<- []byte {
	return r.outgoing
}

func (r *randomLatencyStream) forwardLoop() {
	for {
		select {
		case packet := <-r.outgoing:
			go func() {
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(300)))
				Send(r.Stream, packet)
			}()
		case <-r.Stream.Done():
			return
		}
	}
}
