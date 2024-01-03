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

	server := http.Server{
		Addr:              l.Addr().String(),
		Handler:           http.NewServeMux(),
		ReadHeaderTimeout: 2 * time.Second, // This server is not for production code; picked an arbitrary timeout to statisfy gosec (G112: Potential Slowloris Attack)
	}
	server.Serve(l)
}
