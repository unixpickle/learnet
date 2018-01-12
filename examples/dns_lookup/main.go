// Lookup a host over DNS.

package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/unixpickle/essentials"
	"github.com/unixpickle/learnet/dnsproto"
)

const ServerIP = "8.8.8.8"

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: dns_lookup <hostname>")
		os.Exit(1)
	}
	labels := strings.Split(os.Args[1], ".")

	msg := &dnsproto.Message{
		Header: make(dnsproto.Header, dnsproto.HeaderSize),
		Question: &dnsproto.Question{
			Labels: labels,
			Type:   dnsproto.RecordTypeA,
			Class:  dnsproto.ClassNumberIN,
		},
	}
	msg.Header.SetIdentifier(1337)
	msg.Header.SetQuestionCount(1)
	msg.Header.SetRecursionDesired(true)
	packet, err := msg.Encode()
	essentials.Must(err)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP(ServerIP), Port: 53})
	essentials.Must(err)

	for {
		_, err := conn.Write(packet)
		essentials.Must(err)

		fmt.Println(hex.EncodeToString(packet))

		conn.SetReadDeadline(time.Now().Add(time.Second))
		response := make([]byte, 4096)
		n, err := conn.Read(response)
		if err != nil {
			fmt.Println("retrying after error:", err)
			continue
		}

		decoded, err := dnsproto.DecodeMessage(response[:n])
		essentials.Must(err)

		fmt.Println("got response code:", decoded.Header.ResponseCode())
		for _, record := range decoded.Records {
			fmt.Println("record:", record)
		}
		return
	}
}
