package main

import (
	"net"
	"net/http"
	"strings"
	"time"

	metrics "github.com/docker/go-metrics"
	"github.com/sirupsen/logrus"
)

func startMetricsServer(addr string) error {
	if addr == "" {
		return nil
	}
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
		logrus.Infof("metrics API listening on %s", l.Addr())
		srv := &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Minute, // "G112: Potential Slowloris Attack (gosec)"; not a real concern for our use, so setting a long timeout.
		}
		if err := srv.Serve(l); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			logrus.WithError(err).Error("error serving metrics API")
		}
	}()
	return nil
}
