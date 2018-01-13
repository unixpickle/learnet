// A DNS server that forwards all DNS traffic except for
// special domains which have pre-determined addresses.

package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/unixpickle/essentials"
	"github.com/unixpickle/learnet/dnsproto"
)

var HostServer = &net.UDPAddr{IP: net.IP{8, 8, 8, 8}, Port: 53}

var WriteLock sync.Mutex
var ServerSocket *net.UDPConn

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run main.go <rules.csv>")
		os.Exit(1)
	}
	rules := ReadRules(os.Args[1])

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
			log.Printf("message from %s contains %d questions", addr, len(parsed.Questions))
			for _, query := range parsed.Questions {
				log.Printf("query from %s: %s (%d)", addr, query.Domain, query.Type)
			}
			if resp := RuleResponse(rules, parsed); resp != nil {
				log.Println("faking response to", addr)
				SendResponse(resp, addr)
				continue
			}
		} else {
			log.Printf("%s sent invalid message", addr)
		}

		go ForwardMessage(packet, addr)
	}
}

func ReadRules(path string) map[string]net.IP {
	f, err := os.Open(path)
	essentials.Must(err)
	defer f.Close()
	records, err := csv.NewReader(f).ReadAll()
	essentials.Must(err)
	rules := map[string]net.IP{}
	for _, record := range records {
		parsed := net.ParseIP(record[1])
		if parsed == nil || parsed.To4() == nil {
			essentials.Die("invalid IP:", record[1])
		}
		rules[record[0]] = parsed
	}
	return rules
}

func RuleResponse(rules map[string]net.IP, msg *dnsproto.Message) *dnsproto.Message {
	if len(msg.Questions) != 1 {
		// TODO: support queries with multiple questions.
		return nil
	}
	domainName := strings.ToLower(msg.Questions[0].Domain.String())
	destIP, ok := rules[domainName]
	if !ok {
		return nil
	}
	msg.Header.IsResponse = true
	msg.Header.RecursionAvailable = true
	if msg.Questions[0].Type != dnsproto.RecordTypeA {
		return msg
	}
	msg.Header.AnswerCount = 1
	msg.Records = append(msg.Records, &dnsproto.GenericRecord{
		NameValue:  msg.Questions[0].Domain,
		TypeValue:  dnsproto.RecordTypeA,
		ClassValue: dnsproto.RecordClassIN,
		TTLValue:   30,
		DataValue:  destIP.To4(),
	})
	return msg
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

func SendResponse(resp *dnsproto.Message, addr *net.UDPAddr) {
	encoded, err := resp.Encode()
	if err != nil {
		log.Println(err)
		return
	}
	WriteLock.Lock()
	defer WriteLock.Unlock()
	ServerSocket.WriteToUDP(encoded, addr)
}
