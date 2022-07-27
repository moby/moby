package worker

import (
	"context"

	"github.com/containerd/containerd/content"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/worker/base"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ContainerdWorker is a local worker instance with dedicated snapshotter, cache, and so on.
type ContainerdWorker struct {
	baseWorker *base.Worker
}

// NewContainerdWorker instantiates a local worker
func NewContainerdWorker(ctx context.Context, wo base.WorkerOpt) (*ContainerdWorker, error) {
	bw, err := base.NewWorker(ctx, wo)
	if err != nil {
		return nil, err
	}
	return &ContainerdWorker{
		baseWorker: bw,
	}, nil
}

// ID returns worker ID
func (w *ContainerdWorker) ID() string {
	return w.baseWorker.ID()
}

// Labels returns map of all worker labels
func (w *ContainerdWorker) Labels() map[string]string {
	return w.baseWorker.Labels()
}

// Platforms returns one or more platforms supported by the image.
func (w *ContainerdWorker) Platforms(noCache bool) []ocispec.Platform {
	return w.baseWorker.Platforms(noCache)
}

// GCPolicy returns automatic GC Policy
func (w *ContainerdWorker) GCPolicy() []client.PruneInfo {
	return w.baseWorker.GCPolicy()
}

// ContentStore returns content store
func (w *ContainerdWorker) ContentStore() content.Store {
	return w.baseWorker.ContentStore()
}

// LoadRef loads a reference by ID
func (w *ContainerdWorker) LoadRef(ctx context.Context, id string, hidden bool) (cache.ImmutableRef, error) {
	return w.baseWorker.LoadRef(ctx, id, hidden)
}

// ResolveOp converts a LLB vertex into a LLB operation
func (w *ContainerdWorker) ResolveOp(v solver.Vertex, s frontend.FrontendLLBBridge, sm *session.Manager) (solver.Op, error) {
	return w.baseWorker.ResolveOp(v, s, sm)
}

// ResolveImageConfig returns image config for an image
func (w *ContainerdWorker) ResolveImageConfig(ctx context.Context, ref string, opt llb.ResolveImageConfigOpt, sm *session.Manager, g session.Group) (digest.Digest, []byte, error) {
	return w.baseWorker.ResolveImageConfig(ctx, ref, opt, sm, g)
}

// DiskUsage returns disk usage report
func (w *ContainerdWorker) DiskUsage(ctx context.Context, opt client.DiskUsageInfo) ([]*client.UsageInfo, error) {
	return w.baseWorker.DiskUsage(ctx, opt)
}

// Prune deletes reclaimable build cache
func (w *ContainerdWorker) Prune(ctx context.Context, ch chan client.UsageInfo, info ...client.PruneInfo) error {
	return w.baseWorker.Prune(ctx, ch, info...)
}

// Exporter returns exporter by name
func (w *ContainerdWorker) Exporter(name string, sm *session.Manager) (exporter.Exporter, error) {
	switch name {
	case "moby":
		return w.baseWorker.Exporter(client.ExporterImage, sm)
	default:
		return w.baseWorker.Exporter(name, sm)
	}
}

// PruneCacheMounts removes the current cache snapshots for specified IDs
func (w *ContainerdWorker) PruneCacheMounts(ctx context.Context, ids []string) error {
	return w.baseWorker.PruneCacheMounts(ctx, ids)
}

// FromRemote converts a remote snapshot reference to a local one
func (w *ContainerdWorker) FromRemote(ctx context.Context, remote *solver.Remote) (cache.ImmutableRef, error) {
	return w.baseWorker.FromRemote(ctx, remote)
}

// Executor returns executor.Executor for running processes
func (w *ContainerdWorker) Executor() executor.Executor {
	return w.baseWorker.Executor()
}

// CacheManager returns cache.Manager for accessing local storage
func (w *ContainerdWorker) CacheManager() cache.Manager {
	return w.baseWorker.CacheManager()
}
