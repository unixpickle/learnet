package ipstack

import (
	"bytes"
	"net"
)

type filterStream struct {
	parent   Stream
	incoming <-chan []byte
	outgoing chan<- []byte
}

// Filter wraps a Stream and uses the functions to process
// or drop the packets.
//
// The incoming and outgoing filters take packets and
// return modified packets (or nil to drop the packets).
// If a filter function is nil, the packets go unchanged.
//
// The underlying stream should not be used anymore.
// Rather, all operations should be performed on the
// filtered stream.
func Filter(s Stream, incoming, outgoing func(packet []byte) []byte) Stream {
	res := &filterStream{parent: s, incoming: s.Incoming(), outgoing: s.Outgoing()}

	if incoming != nil {
		ch := make(chan []byte)
		go func() {
			defer close(ch)
			for packet := range s.Incoming() {
				packet = incoming(packet)
				if packet != nil {
					ch <- packet
				}
			}
		}()
		res.incoming = ch
	}

	if outgoing != nil {
		ch := make(chan []byte)
		go func() {
			defer close(s.Outgoing())
			for {
				select {
				case packet, ok := <-ch:
					if !ok {
						return
					}
					packet = outgoing(packet)
					if packet != nil {
						select {
						case s.Outgoing() <- packet:
						case <-s.Done():
							return
						}
					}
				case <-s.Done():
					return
				}
			}
		}()
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

func (f *filterStream) Done() <-chan struct{} {
	return f.parent.Done()
}

// Filter IPv4 packets for a given protocol.
func FilterIPv4Proto(stream Stream, ipProto int) Stream {
	return Filter(stream, func(packet []byte) []byte {
		if len(packet) > 9 && packet[9] == byte(ipProto) {
			return packet
		}
		return nil
	}, nil)
}

// Filter incoming IPv4 packets for a destination address.
func FilterIPv4Dest(stream Stream, dest net.IP) Stream {
	return Filter(stream, func(packet []byte) []byte {
		if len(packet) > 19 && bytes.Equal(packet[16:20], dest[len(dest)-4:]) {
			return packet
		}
		return nil
	}, nil)
}
