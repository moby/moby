package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	p, err := filepath.Abs(filepath.Join("run", "docker", "plugins"))
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(p, 0o755); err != nil {
		panic(err)
	}
	l, err := net.Listen("unix", filepath.Join(p, "basic.sock"))
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	server := http.Server{
		Addr:              l.Addr().String(),
		Handler:           http.NewServeMux(),
		ReadHeaderTimeout: 2 * time.Second, // This server is not for production code; picked an arbitrary timeout to statisfy gosec (G112: Potential Slowloris Attack)
	}
	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1.1+json")
		fmt.Println(w, `{"Implements": ["dummy"]}`)
	})
	server.Serve(l)
}
