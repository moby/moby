package worker

import (
	"context"
	"io"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/leases"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

type Worker interface {
	io.Closer
	// ID needs to be unique in the cluster
	ID() string
	Labels() map[string]string
	Platforms(noCache bool) []ocispecs.Platform
	BuildkitVersion() client.BuildkitVersion

	GCPolicy() []client.PruneInfo
	LoadRef(ctx context.Context, id string, hidden bool) (cache.ImmutableRef, error)
	// ResolveOp resolves Vertex.Sys() to Op implementation.
	ResolveOp(v solver.Vertex, s frontend.FrontendLLBBridge, sm *session.Manager) (solver.Op, error)
	ResolveImageConfig(ctx context.Context, ref string, opt llb.ResolveImageConfigOpt, sm *session.Manager, g session.Group) (digest.Digest, []byte, error)
	DiskUsage(ctx context.Context, opt client.DiskUsageInfo) ([]*client.UsageInfo, error)
	Exporter(name string, sm *session.Manager) (exporter.Exporter, error)
	Prune(ctx context.Context, ch chan client.UsageInfo, opt ...client.PruneInfo) error
	FromRemote(ctx context.Context, remote *solver.Remote) (cache.ImmutableRef, error)
	PruneCacheMounts(ctx context.Context, ids []string) error
	ContentStore() content.Store
	Executor() executor.Executor
	CacheManager() cache.Manager
	LeaseManager() leases.Manager
}

type Infos interface {
	GetDefault() (Worker, error)
	WorkerInfos() []client.WorkerInfo
}
