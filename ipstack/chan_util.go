package ipstack

import (
	"sync"
)

// mergeDones merges done channels.
func mergeDones(chans ...<-chan struct{}) <-chan struct{} {
	if len(chans) == 1 {
		return chans[0]
	} else if len(chans) == 0 {
		return nil
	}

	res := make(chan struct{})
	lock := sync.Mutex{}

	for _, ch := range chans {
		go func(ch <-chan struct{}) {
			select {
			case <-ch:
				lock.Lock()
				defer lock.Unlock()
				select {
				case <-res:
				default:
					close(res)
				}
			case <-res:
			}
		}(ch)
	}

	return res
}

// forwardPackets forwards packets from src to dst and
// stops when cancel is closed.
// It is guaranteed to drop packets that were sent to the
// channel after cancel was closed.
//
// If closeDst is true, then dst is closed on cancel.
func forwardPackets(dst chan<- []byte, src <-chan []byte, closeDst bool,
	cancels ...<-chan struct{}) {
	cancel := mergeDones(cancels...)
	if closeDst {
		defer close(dst)
	}
	for {
		select {
		case incoming, ok := <-src:
			if !ok {
				return
			}
			for _, subCancel := range cancels {
				select {
				case <-subCancel:
					return
				default:
				}
			}
			select {
			case dst <- incoming:
			case <-cancel:
				return
			}
		case <-cancel:
			return
		}
	}
}
