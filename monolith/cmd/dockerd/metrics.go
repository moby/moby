package main

import (
	"net"
	"net/http"

	"github.com/Sirupsen/logrus"
	metrics "github.com/docker/go-metrics"
)

func startMetricsServer(addr string) error {
	if err := allocateDaemonPort(addr); err != nil {
		return err
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	go func() {
		if err := http.Serve(l, mux); err != nil {
			logrus.Errorf("serve metrics api: %s", err)
		}
	}()
	return nil
}
