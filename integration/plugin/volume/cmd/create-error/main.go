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
	server := http.Server{
		Addr:    l.Addr().String(),
		Handler: http.NewServeMux(),
	}
	mux.HandleFunc("/VolumeDriver.Create", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error during create", http.StatusInternalServerError)
	})
	server.Serve(l)
}
