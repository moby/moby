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

	server := http.Server{
		Addr:    l.Addr().String(),
		Handler: http.NewServeMux(),
	}
	server.Serve(l)
}
