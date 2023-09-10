package worker

import (
	"context"

	mobyexporter "github.com/docker/docker/builder/builder-next/exporter"
	"github.com/docker/docker/builder/builder-next/exporter/overrides"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/worker/base"
)

// ContainerdWorker is a local worker instance with dedicated snapshotter, cache, and so on.
type ContainerdWorker struct {
	*base.Worker
}

// NewContainerdWorker instantiates a local worker.
func NewContainerdWorker(ctx context.Context, wo base.WorkerOpt) (*ContainerdWorker, error) {
	bw, err := base.NewWorker(ctx, wo)
	if err != nil {
		return nil, err
	}
	return &ContainerdWorker{Worker: bw}, nil
}

// Exporter returns exporter by name
func (w *ContainerdWorker) Exporter(name string, sm *session.Manager) (exporter.Exporter, error) {
	switch name {
	case mobyexporter.Moby, client.ExporterImage:
		exp, err := w.Worker.Exporter(client.ExporterImage, sm)
		if err != nil {
			return nil, err
		}
		return overrides.NewExporterWrapper(exp)
	default:
		return w.Worker.Exporter(name, sm)
	}
}
