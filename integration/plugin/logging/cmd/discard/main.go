package main

import (
	"net"
	"net/http"
)

func main() {
	l, err := net.Listen("unix", "/run/docker/plugins/plugin.sock")
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	handle(mux)

	server := http.Server{
		Addr:    l.Addr().String(),
		Handler: mux,
	}
	server.Serve(l)
}
