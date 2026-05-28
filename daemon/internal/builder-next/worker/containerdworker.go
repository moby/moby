package worker

import (
	"context"
	nethttp "net/http"

	"github.com/containerd/log"
	"github.com/moby/buildkit/client"
	bkexporter "github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/source/http"
	"github.com/moby/buildkit/worker/base"
	"github.com/moby/moby/v2/daemon/internal/builder-next/exporter"
)

// ContainerdWorker is a local worker instance with dedicated snapshotter, cache, and so on.
type ContainerdWorker struct {
	*base.Worker
	callbacks exporter.BuildkitCallbacks
}

// NewContainerdWorker instantiates a local worker.
func NewContainerdWorker(ctx context.Context, wo base.WorkerOpt, callbacks exporter.BuildkitCallbacks, rt nethttp.RoundTripper) (*ContainerdWorker, error) {
	bw, err := base.NewWorker(ctx, wo)
	if err != nil {
		return nil, err
	}
	hs, err := http.NewSource(http.Opt{
		CacheAccessor: bw.CacheManager(),
		Transport:     rt,
	})
	if err == nil {
		bw.SourceManager.Register(hs)
	} else {
		log.G(ctx).Warnf("Could not register builder http source: %s", err)
	}

	return &ContainerdWorker{Worker: bw, callbacks: callbacks}, nil
}

// Exporter returns exporter by name
func (w *ContainerdWorker) Exporter(name string, sm *session.Manager) (bkexporter.Exporter, error) {
	switch name {
	case exporter.Moby:
		exp, err := w.Worker.Exporter(client.ExporterImage, sm)
		if err != nil {
			return nil, err
		}
		return exporter.NewWrapper(exp, w.ContentStore(), w.callbacks)
	default:
		return w.Worker.Exporter(name, sm)
	}
}
