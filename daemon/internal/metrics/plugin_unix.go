//go:build !windows

package metrics

import (
	"context"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/containerd/log"
	gometrics "github.com/docker/go-metrics"
	"github.com/moby/moby/v2/daemon/pkg/plugin"
	"github.com/moby/moby/v2/pkg/plugingetter"
	"github.com/moby/moby/v2/pkg/plugins"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

const pluginType = "MetricsCollector"

// Plugin represents a metrics collector plugin
type Plugin interface {
	StartMetrics() error
	StopMetrics() error
}

type metricsPluginAdapter struct {
	client *plugins.Client
}

func (a *metricsPluginAdapter) StartMetrics() error {
	return a.client.Call("/MetricsCollector.StartMetrics", nil, nil)
}

func (a *metricsPluginAdapter) StopMetrics() error {
	return a.client.Call("/MetricsCollector.StopMetrics", nil, nil)
}

func makePluginAdapter(p plugingetter.CompatPlugin) (Plugin, error) {
	adapted := p.Client()
	return &metricsPluginAdapter{adapted}, nil
}

// RegisterPlugin starts the metrics server listener and registers the metrics plugin
// callback with the plugin store
func RegisterPlugin(store *plugin.Store, path string) error {
	if err := listen(path); err != nil {
		return err
	}

	store.RegisterRuntimeOpt(pluginType, func(s *specs.Spec) {
		f := plugin.WithSpecMounts([]specs.Mount{
			{Type: "bind", Source: path, Destination: "/run/docker/metrics.sock", Options: []string{"bind", "ro"}},
		})
		f(s)
	})
	store.Handle(pluginType, func(name string, client *plugins.Client) {
		// Use lookup since nothing in the system can really reference it, no need
		// to protect against removal
		p, err := store.Get(name, pluginType, plugingetter.Lookup)
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

	return nil
}

// CleanupPlugin stops metrics collection for all plugins
func CleanupPlugin(store plugingetter.PluginGetter) {
	ls := store.GetAllManagedPluginsByCap(pluginType)
	var wg sync.WaitGroup
	wg.Add(len(ls))

	for _, plugin := range ls {
		p := plugin
		go func() {
			defer wg.Done()

			adapter, err := makePluginAdapter(p)
			if err != nil {
				log.G(context.TODO()).WithError(err).WithField("plugin", p.Name()).Error("Error creating metrics plugin adapter")
				return
			}
			if err := adapter.StopMetrics(); err != nil {
				log.G(context.TODO()).WithError(err).WithField("plugin", p.Name()).Error("Error stopping plugin metrics collection")
			}
		}()
	}
	wg.Wait()

	if listener != nil {
		_ = listener.Close()
	}
}

var listener net.Listener

func listen(path string) error {
	_ = os.Remove(path)
	l, err := net.Listen("unix", path)
	if err != nil {
		return errors.Wrap(err, "error setting up metrics plugin listener")
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", gometrics.Handler())
	go func() {
		log.G(context.TODO()).Debugf("metrics API listening on %s", l.Addr())
		srv := &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Minute, // "G112: Potential Slowloris Attack (gosec)"; not a real concern for our use, so setting a long timeout.
		}
		if err := srv.Serve(l); err != nil && !errors.Is(err, net.ErrClosed) {
			log.G(context.TODO()).WithError(err).Error("error serving metrics API")
		}
	}()
	listener = l
	return nil
}
