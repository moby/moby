package command

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/containerd/log"
	gometrics "github.com/docker/go-metrics"
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
	mux.Handle("/metrics", gometrics.Handler())
	go func() {
		log.G(context.TODO()).Infof("metrics API listening on %s", l.Addr())
		srv := &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Minute, // "G112: Potential Slowloris Attack (gosec)"; not a real concern for our use, so setting a long timeout.
		}
		if err := srv.Serve(l); err != nil && !errors.Is(err, net.ErrClosed) {
			log.G(context.TODO()).WithError(err).Error("error serving metrics API")
		}
	}()
	return nil
}
