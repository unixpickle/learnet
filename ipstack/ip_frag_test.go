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
	sender, receiver := Pipe(1000, 1000)
	sender = &randomLatencyStream{Stream: sender}
	sender = AddIPv4Identifiers(FragmentOutgoingIPv4(sender, 133))
	receiver = DefragmentIncomingIPv4(receiver, time.Second*3)

	packets := make([][]byte, 30)
	for i := range packets {
		payload := make([]byte, rand.Intn(300)+30)
		for j := 0; j < len(payload); j++ {
			payload[j] = byte(rand.Intn(0x100))
		}
		packets[i] = NewIPv4Packet(rand.Intn(30)+1, ProtocolNumberICMP, net.IP{10, 0, 0, 1},
			net.IP{10, 0, 0, 2}, payload)
		if err := sender.Write(packets[i]); err != nil {
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
}

func (r *randomLatencyStream) Write(packet []byte) error {
	go func() {
		time.Sleep(time.Millisecond * time.Duration(rand.Intn(300)))
		r.Stream.Write(packet)
	}()
	return nil
}
