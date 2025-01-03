package worker

import (
	"context"

	"github.com/docker/docker/builder/builder-next/exporter"
	"github.com/moby/buildkit/client"
	bkexporter "github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/worker/base"
)

// ContainerdWorker is a local worker instance with dedicated snapshotter, cache, and so on.
type ContainerdWorker struct {
	*base.Worker
	callbacks     exporter.BuildkitCallbacks
	exporterAttrs map[string]string
}

// NewContainerdWorker instantiates a local worker.
func NewContainerdWorker(ctx context.Context, wo base.WorkerOpt, callbacks exporter.BuildkitCallbacks, exporterAttrs map[string]string) (*ContainerdWorker, error) {
	bw, err := base.NewWorker(ctx, wo)
	if err != nil {
		return nil, err
	}
	return &ContainerdWorker{Worker: bw, callbacks: callbacks, exporterAttrs: exporterAttrs}, nil
}

// Exporter returns exporter by name
func (w *ContainerdWorker) Exporter(name string, sm *session.Manager) (bkexporter.Exporter, error) {
	switch name {
	case exporter.Moby:
		exp, err := w.Worker.Exporter(client.ExporterImage, sm)
		if err != nil {
			return nil, err
		}
		return exporter.NewWrapper(exp, w.callbacks, w.exporterAttrs)
	default:
		return w.Worker.Exporter(name, sm)
	}
}
