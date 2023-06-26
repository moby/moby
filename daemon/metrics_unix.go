//go:build !windows

package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/daemon/config"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/plugin"
	metrics "github.com/docker/go-metrics"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func (daemon *Daemon) listenMetricsSock(cfg *config.Config) (string, error) {
	path := filepath.Join(cfg.ExecRoot, "metrics.sock")
	unix.Unlink(path)
	l, err := net.Listen("unix", path)
	if err != nil {
		return "", errors.Wrap(err, "error setting up metrics plugin listener")
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	go func() {
		log.G(context.TODO()).Debugf("metrics API listening on %s", l.Addr())
		srv := &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Minute, // "G112: Potential Slowloris Attack (gosec)"; not a real concern for our use, so setting a long timeout.
		}
		if err := srv.Serve(l); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			log.G(context.TODO()).WithError(err).Error("error serving metrics API")
		}
	}()
	daemon.metricsPluginListener = l
	return path, nil
}

func registerMetricsPluginCallback(store *plugin.Store, sockPath string) {
	store.RegisterRuntimeOpt(metricsPluginType, func(s *specs.Spec) {
		f := plugin.WithSpecMounts([]specs.Mount{
			{Type: "bind", Source: sockPath, Destination: "/run/docker/metrics.sock", Options: []string{"bind", "ro"}},
		})
		f(s)
	})
	store.Handle(metricsPluginType, func(name string, client *plugins.Client) {
		// Use lookup since nothing in the system can really reference it, no need
		// to protect against removal
		p, err := store.Get(name, metricsPluginType, plugingetter.Lookup)
		if err != nil {
			return
		}

		adapter, err := makePluginAdapter(p)
		if err != nil {
			log.G(context.TODO()).WithError(err).WithField("plugin", p.Name()).Error("Error creating plugin adapter")
		}
		if err := adapter.StartMetrics(); err != nil {
			log.G(context.TODO()).WithError(err).WithField("plugin", p.Name()).Error("Error starting metrics collector plugin")
		}
	})
}
