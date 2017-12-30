package tunnet

import "github.com/unixpickle/learnet/ipstack"

type tunnelStream struct {
	tunnel   Tunnel
	incoming <-chan []byte
	outgoing chan<- []byte
	done     <-chan struct{}
}

// TunnelStream creates a packet stream for the tunnel.
//
// The stream buffers I/O operations up to a point, and
// then blocks if necessary.
//
// When the stream is closed, it closes the underlying
// tunnel automatically.
func TunnelStream(t Tunnel, readBuffer, writeBuffer int) ipstack.Stream {
	incoming := make(chan []byte, readBuffer)
	outgoing := make(chan []byte, writeBuffer)
	done := make(chan struct{})

	res := &tunnelStream{tunnel: t, incoming: incoming, outgoing: outgoing, done: done}
	go res.readLoop(done, incoming)
	go res.writeLoop(outgoing)

	return res
}

func (t *tunnelStream) Incoming() <-chan []byte {
	return t.incoming
}

func (t *tunnelStream) Write(packet []byte) error {
	select {
	case <-t.done:
		return ipstack.WriteClosedErr
	default:
	}

	select {
	case t.outgoing <- packet:
		return nil
	case <-t.done:
		return ipstack.WriteClosedErr
	default:
		return ipstack.WriteBufferFullErr
	}
}

func (t *tunnelStream) Close() error {
	select {
	case <-t.done:
		return ipstack.AlreadyClosedErr
	default:
	}
	t.tunnel.Close()
	<-t.done
	return nil
}

func (t *tunnelStream) Done() <-chan struct{} {
	return t.done
}

func (t *tunnelStream) readLoop(done chan<- struct{}, incoming chan<- []byte) {
	defer close(done)
	defer close(incoming)
	for {
		packet, err := t.tunnel.ReadPacket()
		if err != nil {
			return
		}
		select {
		case incoming <- packet:
		default:
		}
	}
}

func (t *tunnelStream) writeLoop(outgoing <-chan []byte) {
	defer t.tunnel.Close()
	for {
		select {
		case packet, ok := <-outgoing:
			if !ok {
				panic("outgoing channel closed")
			}
			t.tunnel.WritePacket(packet)
		case <-t.done:
			return
		}
	}
}
