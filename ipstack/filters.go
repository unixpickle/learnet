package ipstack

// A FilterFunc modifies or drops packets.
//
// The function may modify a packet or create and return
// an entirely new packet.
// If the function returns nil, then the packet is
// dropped altogether.
type FilterFunc func(packet []byte) []byte

type filterStream struct {
	Stream
	incoming <-chan []byte
	outgoing chan<- []byte
}

// Filter wraps a Stream and uses the functions to process
// or drop the packets.
//
// The underlying stream should not be used anymore.
// Rather, all operations should be performed on the
// filtered stream.
func Filter(s Stream, incoming, outgoing FilterFunc) Stream {
	res := &filterStream{Stream: s, incoming: s.Incoming(), outgoing: s.Outgoing()}

	if incoming != nil {
		ch := make(chan []byte)
		go filterChan(incoming, ch, s.Incoming(), true, s.Done())
		res.incoming = ch
	}

	if outgoing != nil {
		ch := make(chan []byte)
		go filterChan(outgoing, s.Outgoing(), ch, false, s.Done())
		res.outgoing = ch
	}

	return res
}

func (f *filterStream) Incoming() <-chan []byte {
	return f.incoming
}

func (f *filterStream) Outgoing() chan<- []byte {
	return f.outgoing
}

func filterChan(filter FilterFunc, dst chan<- []byte, src <-chan []byte, closeDst bool,
	done <-chan struct{}) {
	if closeDst {
		defer close(dst)
	}
	for {
		select {
		case packet, ok := <-src:
			if !ok {
				return
			}
			packet = filter(packet)
			if packet == nil {
				continue
			}
			select {
			case dst <- packet:
			case <-done:
				return
			}
		case <-done:
			return
		}
	}
}
