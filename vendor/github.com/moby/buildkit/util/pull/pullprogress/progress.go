package pullprogress

import (
	"context"
	"io"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/remotes"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/progress"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type PullManager interface {
	content.IngestManager
	content.Manager
}

type ProviderWithProgress struct {
	Provider content.Provider
	Manager  PullManager
}

func (p *ProviderWithProgress) ReaderAt(ctx context.Context, desc ocispecs.Descriptor) (content.ReaderAt, error) {
	ra, err := p.Provider.ReaderAt(ctx, desc)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancelCause(ctx)
	doneCh := make(chan struct{})
	go trackProgress(ctx, desc, p.Manager, doneCh)
	return readerAtWithCancel{ReaderAt: ra, cancel: cancel, doneCh: doneCh, logger: bklog.G(ctx)}, nil
}

type readerAtWithCancel struct {
	content.ReaderAt
	cancel func(error)
	doneCh <-chan struct{}
	logger *logrus.Entry
}

func (ra readerAtWithCancel) Close() error {
	ra.cancel(errors.WithStack(context.Canceled))
	select {
	case <-ra.doneCh:
	case <-time.After(time.Second):
		ra.logger.Warn("timeout waiting for pull progress to complete")
	}
	return ra.ReaderAt.Close()
}

type FetcherWithProgress struct {
	Fetcher remotes.Fetcher
	Manager PullManager
}

func (f *FetcherWithProgress) Fetch(ctx context.Context, desc ocispecs.Descriptor) (io.ReadCloser, error) {
	rc, err := f.Fetcher.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancelCause(ctx)
	doneCh := make(chan struct{})
	go trackProgress(ctx, desc, f.Manager, doneCh)
	return readerWithCancel{ReadCloser: rc, cancel: cancel, doneCh: doneCh, logger: bklog.G(ctx)}, nil
}

type readerWithCancel struct {
	io.ReadCloser
	cancel func(error)
	doneCh <-chan struct{}
	logger *logrus.Entry
}

func (r readerWithCancel) Close() error {
	r.cancel(errors.WithStack(context.Canceled))
	select {
	case <-r.doneCh:
	case <-time.After(time.Second):
		r.logger.Warn("timeout waiting for pull progress to complete")
	}
	return r.ReadCloser.Close()
}

func trackProgress(ctx context.Context, desc ocispecs.Descriptor, manager PullManager, doneCh chan<- struct{}) {
	defer close(doneCh)

	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	go func(ctx context.Context) {
		<-ctx.Done()
		ticker.Stop()
	}(ctx)

	pw, _, _ := progress.NewFromContext(ctx)
	defer pw.Close()

	ingestRef := remotes.MakeRefKey(ctx, desc)

	started := time.Now()
	onFinalStatus := false
	for !onFinalStatus {
		select {
		case <-ctx.Done():
			onFinalStatus = true
			// we need a context for the manager.Status() calls to pass once. after that this function will exit
			ctx = context.TODO()
		case <-ticker.C:
		}

		status, err := manager.Status(ctx, ingestRef)
		if err == nil {
			pw.Write(desc.Digest.String(), progress.Status{
				Current: int(status.Offset),
				Total:   int(status.Total),
				Started: &started,
			})
			continue
		} else if !errors.Is(err, errdefs.ErrNotFound) {
			bklog.G(ctx).Errorf("unexpected error getting ingest status of %q: %v", ingestRef, err)
			return
		}

		info, err := manager.Info(ctx, desc.Digest)
		if err == nil {
			// info.CreatedAt could be before started if parallel pull just completed
			if info.CreatedAt.Before(started) {
				started = info.CreatedAt
			}
			pw.Write(desc.Digest.String(), progress.Status{
				Current:   int(info.Size),
				Total:     int(info.Size),
				Started:   &started,
				Completed: &info.CreatedAt,
			})
			return
		}
	}
}
