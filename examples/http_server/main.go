// Demonstrate how to use the TCP implementation under the
// Go http server.
//
// Run this as root.

package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"

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

	fmt.Printf("Connect to http://%s:1337 to browse files!\n", examples.Gateway)

	http.HandleFunc("/", HandleRequest)
	essentials.Must(http.Serve(listener, nil))

	select {}
}

func HandleRequest(w http.ResponseWriter, r *http.Request) {
	localPath := filepath.FromSlash(r.URL.Path)
	if stat, err := os.Stat(localPath); err != nil {
		http.Error(w, "not found", 404)
		return
	} else if stat.IsDir() {
		listing, err := ioutil.ReadDir(localPath)
		if err != nil {
			http.Error(w, "cannot read", 404)
			return
		}
		res := "<!doctype html><html><body>"
		for _, entry := range listing {
			fullPath := filepath.Join(localPath, entry.Name())
			res += "<a href=\"" + filepath.ToSlash(fullPath) + "\">" + entry.Name() + "</a><br>"
		}
		res += "</body></html>"
		w.Write([]byte(res))
	} else {
		http.ServeFile(w, r, localPath)
	}
}
