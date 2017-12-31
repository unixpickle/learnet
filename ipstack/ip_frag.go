package ipstack

import (
	"bytes"
	"errors"
	"net"
	"sort"
	"time"

	"github.com/unixpickle/essentials"
)

// DefaultDefragmentTimeout is the default amount of time
// fragments of a packet should be kept around before the
// packet is dropped.
const DefaultDefragmentTimeout = time.Second

// DefragmentIncomingIPv4 reassembles incoming fragmented
// IPv4 packets.
//
// The timeout indicates how long to keep fragmented
// packets around before giving up on them.
// If 0 is passed, DefaultDefragmentTimeout is used.
//
// All incoming packets are assumed to be valid.
func DefragmentIncomingIPv4(stream Stream, timeout time.Duration) Stream {
	if timeout == 0 {
		timeout = DefaultDefragmentTimeout
	}
	defrag := &ipv4Defragmenter{timeout: int64(timeout / time.Nanosecond)}
	return Filter(stream, func(packet []byte) []byte {
		ipPacket := IPv4Packet(packet)
		_, more, offset := ipPacket.FragmentInfo()
		if more || offset != 0 {
			return defrag.AddPacket(ipPacket)
		}
		return packet
	}, nil)
}

// FragmentOutgoingIPv4 splits large outgoing packets into
// fragments.
//
// The mtu argument specifies the maximum packet size.
//
// All outgoing packets should already have unique
// identifiers, e.g. from AddIPv4Identifiers().
//
// All outgoing packets are assumed to be valid.
func FragmentOutgoingIPv4(stream Stream, mtu int) Stream {
	return &ipv4Fragmenter{Stream: stream, MTU: mtu}
}

type ipv4Fragmenter struct {
	Stream
	MTU int
}

func (i *ipv4Fragmenter) Write(packet []byte) error {
	if len(packet) < i.MTU {
		return i.Stream.Write(packet)
	}
	ipPacket := IPv4Packet(packet)
	header := ipPacket.Header()
	payload := ipPacket.Payload()

	dontFrag, more, existingOffset := ipPacket.FragmentInfo()
	if dontFrag {
		return errors.New("write: packet cannot be fragmented")
	} else if more || existingOffset != 0 {
		return errors.New("write: packet is already fragmented")
	}

	maxPayload := i.MTU - len(header)
	maxPayload ^= maxPayload & 7

	if maxPayload == 0 {
		return errors.New("write: no room for payload")
	}

	offset := 0
	for len(payload)-offset > 0 {
		chunkSize := essentials.MinInt(maxPayload, len(payload)-offset)
		next := append(append(IPv4Packet{}, header...), payload[offset:offset+chunkSize]...)
		next.SetFragmentInfo(false, chunkSize+offset < len(payload), offset>>3)
		next.SetTotalLength()
		next.SetChecksum()
		if err := i.Stream.Write(next); err != nil {
			return err
		}
		offset += chunkSize
	}

	return nil
}

// A ipv4Defragmenter tracks the states of packet
// reconstructions.
type ipv4Defragmenter struct {
	timeout         int64
	reconstructions []*ipv4Reconstruction
}

// AddPacket adds a packet to a reconstruction.
//
// If the packet is reconstructed, it is returned.
// Otherwise, nil is returned.
func (i *ipv4Defragmenter) AddPacket(p IPv4Packet) IPv4Packet {
	i.dropOld()
	source := p.SourceAddr()
	for j, recon := range i.reconstructions {
		if recon.Identification == p.Identification() && bytes.Equal(source, recon.Source) {
			recon.AddFragment(p)
			if recon.Ready() {
				essentials.OrderedDelete(&i.reconstructions, j)
				return recon.Reassemble()
			}
			return nil
		}
	}
	i.reconstructions = append(i.reconstructions, &ipv4Reconstruction{
		DropTime:       time.Now().UnixNano() + i.timeout,
		Identification: p.Identification(),
		Source:         p.SourceAddr(),
		Fragments:      []IPv4Packet{p},
	})
	return nil
}

func (i *ipv4Defragmenter) dropOld() {
	curTime := time.Now().UnixNano()
	for j := 0; j < len(i.reconstructions); j++ {
		recon := i.reconstructions[j]
		if curTime >= recon.DropTime {
			essentials.OrderedDelete(&i.reconstructions, j)
			j--
			continue
		}
	}
}

// ipv4Reconstruction tracks the state of a fragmented
// IPv4 packet as its parts are received.
type ipv4Reconstruction struct {
	DropTime       int64
	Identification uint16
	Source         net.IP
	Fragments      []IPv4Packet
}

// AddFragment adds a fragment to the buffer.
func (i *ipv4Reconstruction) AddFragment(p IPv4Packet) {
	_, _, off := p.FragmentInfo()
	idx := sort.Search(len(i.Fragments), func(idx int) bool {
		_, _, off1 := i.Fragments[idx].FragmentInfo()
		return off < off1
	})
	if idx == len(i.Fragments) {
		i.Fragments = append(i.Fragments, p)
		return
	}
	_, _, off1 := i.Fragments[idx].FragmentInfo()
	if off1 == off {
		// Received a packet twice.
		return
	}
	i.Fragments = append(i.Fragments, nil)
	copy(i.Fragments[idx+1:], i.Fragments[idx:])
	i.Fragments[idx] = p
}

// Ready checks if the packet has been reassembled.
func (i *ipv4Reconstruction) Ready() bool {
	_, more, _ := i.Fragments[len(i.Fragments)-1].FragmentInfo()
	if more {
		return false
	}

	nextOff := 0
	for _, frag := range i.Fragments {
		_, _, off := frag.FragmentInfo()
		if off<<3 != nextOff {
			return false
		}
		nextOff += len(frag.Payload())
	}

	return true
}

// Reassemble assembles the full packet.
//
// This assumes that the packet is ready.
func (i *ipv4Reconstruction) Reassemble() IPv4Packet {
	packet := append(IPv4Packet{}, i.Fragments[0].Header()...)
	for _, frag := range i.Fragments {
		packet = append(packet, frag.Payload()...)
	}
	packet.SetFragmentInfo(false, false, 0)
	packet.SetTotalLength()
	packet.SetChecksum()
	return packet
}
