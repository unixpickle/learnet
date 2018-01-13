// A DNS server that forwards all DNS traffic but
// passively logs all queries.

package main

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/unixpickle/essentials"
	"github.com/unixpickle/learnet/dnsproto"
)

var HostServer = &net.UDPAddr{IP: net.IP{8, 8, 8, 8}, Port: 53}

var WriteLock sync.Mutex
var ServerSocket *net.UDPConn

func main() {
	var err error
	ServerSocket, err = net.ListenUDP("udp", &net.UDPAddr{Port: 53})
	essentials.Must(err)
	defer ServerSocket.Close()

	for {
		packet := make([]byte, 4096)
		n, addr, err := ServerSocket.ReadFromUDP(packet)
		essentials.Must(err)
		packet = packet[:n]

		if parsed, err := dnsproto.DecodeMessage(packet); err == nil {
			for _, query := range parsed.Questions {
				log.Printf("query from %s: %s (%d)", addr, query.Domain, query.Type)
			}
		} else {
			log.Printf("%s sent invalid message", addr)
		}

		go ForwardMessage(packet, addr)
	}
}

func ForwardMessage(packet []byte, addr *net.UDPAddr) {
	conn, err := net.DialUDP("udp", nil, HostServer)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()
	conn.Write(packet)
	conn.SetReadDeadline(time.Now().Add(time.Second * 10))

	response := make([]byte, 4096)
	n, err := conn.Read(response)
	if err != nil {
		return
	}

	log.Println("got response for", addr)
	WriteLock.Lock()
	defer WriteLock.Unlock()
	ServerSocket.WriteToUDP(response[:n], addr)
}
