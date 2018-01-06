// +build darwin

package tunnet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/unixpickle/essentials"

	"golang.org/x/sys/unix"
)

// routeLock prevents multiple routing requests from
// running in the same process at once, since PID is all
// we use to identify responses.
var routeLock sync.Mutex

// AddRoute adds an entry to the system routing table.
func AddRoute(destination, gateway net.IP, mask net.IPMask) (err error) {
	defer essentials.AddCtxTo("add route", &err)
	if destination.To4() == nil || gateway.To4() == nil || len(mask) != 4 {
		return errors.New("only IPv4 is supported")
	}
	header := &routeMsgHeader{
		Type:  routeMsgTypeAdd,
		Addrs: routeMsgAddrDst | routeMsgAddrGateway | routeMsgAddrNetmask,
	}
	var body bytes.Buffer
	for _, ip := range [][]byte{destination, gateway, mask} {
		body.Write(packSockaddr4(ip, 0))
	}
	return runRouteMsg(header, body.Bytes())
}

func runRouteMsg(header *routeMsgHeader, body []byte) error {
	routeLock.Lock()
	defer routeLock.Unlock()

	sock, err := unix.Socket(pfRoute, unix.SOCK_RAW, 0)
	if err != nil {
		return err
	}
	defer unix.Close(sock)

	header.MessageLen = uint16(routeMsgHeaderLen + len(body))
	header.Version = routeMsgVersion
	header.Seq = 1337

	var buf bytes.Buffer
	buf.Write(header.Encode())
	buf.Write(body)
	n, err := unix.Write(sock, buf.Bytes())
	if err != nil {
		return err
	} else if n != buf.Len() {
		return errors.New("unexpected write size")
	}

	// TODO: read response here...
	return nil
}

const (
	routeMsgVersion   = 5
	routeMsgHeaderLen = 92
)

type routeMsgType uint8

const (
	routeMsgTypeAdd    routeMsgType = 1
	routeMsgTypeDelete              = 2
)

type routeMsgFlag int32

type routeMsgAddr int32

const (
	routeMsgAddrDst     routeMsgAddr = 1
	routeMsgAddrGateway              = 2
	routeMsgAddrNetmask              = 4
	routeMsgMetricsSize              = 56
)

type routeMsgHeader struct {
	MessageLen uint16
	Version    uint8
	Type       routeMsgType
	Index      uint16
	Flags      routeMsgFlag
	Addrs      routeMsgAddr
	Pid        int32
	Seq        int32
	Errno      int32
	Use        int32
	Inits      uint32
	Metrics    [routeMsgMetricsSize]byte
}

func (r *routeMsgHeader) Encode() []byte {
	var buf bytes.Buffer
	binary.Write(&buf, systemByteOrder, r.MessageLen)
	binary.Write(&buf, systemByteOrder, r.Version)
	binary.Write(&buf, systemByteOrder, r.Type)
	binary.Write(&buf, systemByteOrder, r.Index)
	binary.Write(&buf, systemByteOrder, uint16(0))
	binary.Write(&buf, systemByteOrder, r.Flags)
	binary.Write(&buf, systemByteOrder, r.Addrs)
	binary.Write(&buf, systemByteOrder, r.Pid)
	binary.Write(&buf, systemByteOrder, r.Seq)
	binary.Write(&buf, systemByteOrder, r.Errno)
	binary.Write(&buf, systemByteOrder, r.Use)
	binary.Write(&buf, systemByteOrder, r.Inits)
	buf.Write(r.Metrics[:])
	return buf.Bytes()
}

func (r *routeMsgHeader) Decode(header []byte) {
	buf := bytes.NewReader(header)
	binary.Read(buf, systemByteOrder, &r.MessageLen)
	binary.Read(buf, systemByteOrder, &r.Version)
	binary.Read(buf, systemByteOrder, &r.Type)
	binary.Read(buf, systemByteOrder, &r.Index)
	buf.Seek(2, io.SeekCurrent)
	binary.Read(buf, systemByteOrder, &r.Flags)
	binary.Read(buf, systemByteOrder, &r.Addrs)
	binary.Read(buf, systemByteOrder, &r.Pid)
	binary.Read(buf, systemByteOrder, &r.Seq)
	binary.Read(buf, systemByteOrder, &r.Errno)
	binary.Read(buf, systemByteOrder, &r.Use)
	binary.Read(buf, systemByteOrder, &r.Inits)
	buf.Read(r.Metrics[:])
}
