package main

import (
	"net"
	"net/http"
	"time"
)

func main() {
	l, err := net.Listen("unix", "/run/docker/plugins/plugin.sock")
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	handle(mux)

	server := http.Server{
		Addr:              l.Addr().String(),
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second, // This server is not for production code; picked an arbitrary timeout to satisfy gosec (G112: Potential Slowloris Attack)
	}
	server.Serve(l)
}
