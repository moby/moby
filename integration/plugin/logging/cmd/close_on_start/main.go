package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

type start struct {
	File string
}

func main() {
	l, err := net.Listen("unix", "/run/docker/plugins/plugin.sock")
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/LogDriver.StartLogging", func(w http.ResponseWriter, req *http.Request) {
		startReq := &start{}
		if err := json.NewDecoder(req.Body).Decode(startReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		f, err := os.OpenFile(startReq.File, os.O_RDONLY, 0600)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Close the file immediately, this allows us to test what happens in the daemon when the plugin has closed the
		// file or, for example, the plugin has crashed.
		f.Close()

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{}`)
	})
	server := http.Server{
		Addr:              l.Addr().String(),
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second, // This server is not for production code; picked an arbitrary timeout to statisfy gosec (G112: Potential Slowloris Attack)
	}

	server.Serve(l)
}
