package worker

import (
	"context"

	mobyexporter "github.com/docker/docker/builder/builder-next/exporter"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/worker/base"
)

// ContainerdWorker is a local worker instance with dedicated snapshotter, cache, and so on.
type ContainerdWorker struct {
	*base.Worker
	callback mobyexporter.ImageExportedByBuildkit
}

// NewContainerdWorker instantiates a local worker.
func NewContainerdWorker(ctx context.Context, wo base.WorkerOpt, callback mobyexporter.ImageExportedByBuildkit) (*ContainerdWorker, error) {
	bw, err := base.NewWorker(ctx, wo)
	if err != nil {
		return nil, err
	}
	return &ContainerdWorker{Worker: bw, callback: callback}, nil
}

// Exporter returns exporter by name
func (w *ContainerdWorker) Exporter(name string, sm *session.Manager) (exporter.Exporter, error) {
	switch name {
	case mobyexporter.Moby:
		exp, err := w.Worker.Exporter(client.ExporterImage, sm)
		if err != nil {
			return nil, err
		}
		return mobyexporter.NewWrapper(exp, w.callback)
	default:
		return w.Worker.Exporter(name, sm)
	}
}
