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
	incoming       <-chan []byte
	outgoingFilter FilterFunc
}

// Filter wraps a Stream and uses the functions to process
// or drop the packets.
//
// The underlying stream should not be used anymore.
// Rather, all operations should be performed on the
// filtered stream.
func Filter(s Stream, incoming, outgoing FilterFunc) Stream {
	res := &filterStream{Stream: s, incoming: s.Incoming(), outgoingFilter: outgoing}

	if incoming != nil {
		ch := make(chan []byte)
		go func() {
			defer close(ch)
			for packet := range s.Incoming() {
				packet = incoming(packet)
				if packet != nil {
					select {
					case ch <- packet:
					case <-s.Done():
						return
					}
				}
			}
		}()
		res.incoming = ch
	}

	return res
}

func (f *filterStream) Incoming() <-chan []byte {
	return f.incoming
}

func (f *filterStream) Write(buf []byte) error {
	if f.outgoingFilter != nil {
		buf = f.outgoingFilter(buf)
		if buf == nil {
			return nil
		}
	}
	return f.Stream.Write(buf)
}
