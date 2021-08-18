package worker

import (
	"context"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/compression"
)

func NewWorkerRefResult(ref cache.ImmutableRef, worker Worker) solver.Result {
	return &workerRefResult{&WorkerRef{ImmutableRef: ref, Worker: worker}}
}

type WorkerRef struct {
	ImmutableRef cache.ImmutableRef
	Worker       Worker
}

func (wr *WorkerRef) ID() string {
	refID := ""
	if wr.ImmutableRef != nil {
		refID = wr.ImmutableRef.ID()
	}
	return wr.Worker.ID() + "::" + refID
}

// GetRemote method abstracts ImmutableRef's GetRemote to allow a Worker to override.
// This is needed for moby integration.
// Use this method instead of calling ImmutableRef.GetRemote() directly.
func (wr *WorkerRef) GetRemote(ctx context.Context, createIfNeeded bool, compressionType compression.Type, forceCompression bool, g session.Group) (*solver.Remote, error) {
	if w, ok := wr.Worker.(interface {
		GetRemote(context.Context, cache.ImmutableRef, bool, compression.Type, bool, session.Group) (*solver.Remote, error)
	}); ok {
		return w.GetRemote(ctx, wr.ImmutableRef, createIfNeeded, compressionType, forceCompression, g)
	}
	return wr.ImmutableRef.GetRemote(ctx, createIfNeeded, compressionType, forceCompression, g)
}

type workerRefResult struct {
	*WorkerRef
}

func (r *workerRefResult) Release(ctx context.Context) error {
	if r.ImmutableRef == nil {
		return nil
	}
	return r.ImmutableRef.Release(ctx)
}

func (r *workerRefResult) Sys() interface{} {
	return r.WorkerRef
}

func (r *workerRefResult) Clone() solver.Result {
	r2 := *r
	if r.ImmutableRef != nil {
		r.ImmutableRef = r.ImmutableRef.Clone()
	}
	return &r2
}
