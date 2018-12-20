// Demonstrate the basic setup for TCP servers.
//
// Run this as root.

package main

import (
	"fmt"
	"log"
	"net"

	"github.com/unixpickle/essentials"
	"github.com/unixpickle/learnet/examples"
	"github.com/unixpickle/learnet/ipstack"
)

const BufferSize = 10

func main() {
	multi := examples.SetupTunnel()
	examples.SetupIPServices(multi)

	tcpStream, err := multi.Fork(BufferSize)
	essentials.Must(err)
	tcpNet := ipstack.NewTCP4Net(tcpStream, examples.Gateway, nil, 0)

	listener, err := tcpNet.ListenTCP(&net.TCPAddr{IP: examples.Gateway, Port: 1337})
	essentials.Must(err)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatal(err)
			}
			go func() {
				defer conn.Close()
				fmt.Println("Handling conn from", conn.RemoteAddr())
				var data [1]byte
				for {
					n, err := conn.Read(data[:])
					if err != nil {
						fmt.Println("got read error:", err)
						return
					}
					conn.Write(data[:n])
					if n == 1 && data[0] == '!' {
						fmt.Println("terminating on '!'")
						return
					}
				}
			}()
		}
	}()

	fmt.Printf("Send TCP traffic to %s:1337!\n", examples.Gateway)

	select {}
}
