package worker

import (
	"context"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/solver"
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
