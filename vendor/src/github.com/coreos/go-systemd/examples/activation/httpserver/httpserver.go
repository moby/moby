package main

import (
	"io"
	"net"
	"net/http"

	"github.com/coreos/go-systemd/activation"
)

func HelloServer(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "hello socket activated world!\n")
}

func main() {
	files := activation.Files(true)

	if len(files) != 1 {
		panic("Unexpected number of socket activation fds")
	}

	l, err := net.FileListener(files[0])
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/", HelloServer)
	http.Serve(l, nil)
}
