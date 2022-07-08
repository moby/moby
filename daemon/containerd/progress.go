package containerd

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/stringid"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

type progressUpdater interface {
	UpdateProgress(context.Context, *jobs, progress.Output, time.Time) error
}

type jobs struct {
	descs map[digest.Digest]ocispec.Descriptor
	mu    sync.Mutex
}

// newJobs creates a new instance of the job status tracker
func newJobs() *jobs {
	return &jobs{
		descs: map[digest.Digest]ocispec.Descriptor{},
	}
}

func (j *jobs) showProgress(ctx context.Context, out progress.Output, updater progressUpdater) func() {
	ctx, cancelProgress := context.WithCancel(ctx)

	start := time.Now()
	lastUpdate := make(chan struct{})

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := updater.UpdateProgress(ctx, j, out, start); err != nil {
					if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
						logrus.WithError(err).Error("Updating progress failed")
					}
				}
			case <-ctx.Done():
				ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
				defer cancel()
				updater.UpdateProgress(ctx, j, out, start)
				close(lastUpdate)
				return
			}
		}
	}()

	return func() {
		cancelProgress()
		// Wait for the last update to finish.
		// UpdateProgress may still write progress to output and we need
		// to keep the caller from closing it before we finish.
		<-lastUpdate
	}
}

// Add adds a descriptor to be tracked
func (j *jobs) Add(desc ocispec.Descriptor) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if _, ok := j.descs[desc.Digest]; ok {
		return
	}
	j.descs[desc.Digest] = desc
}

// Remove removes a descriptor
func (j *jobs) Remove(desc ocispec.Descriptor) {
	j.mu.Lock()
	defer j.mu.Unlock()

	delete(j.descs, desc.Digest)
}

// Jobs returns a list of all tracked descriptors
func (j *jobs) Jobs() []ocispec.Descriptor {
	j.mu.Lock()
	defer j.mu.Unlock()

	descs := make([]ocispec.Descriptor, 0, len(j.descs))
	for _, d := range j.descs {
		descs = append(descs, d)
	}
	return descs
}

type pullProgress struct {
	Store      content.Store
	ShowExists bool
}

func (p pullProgress) UpdateProgress(ctx context.Context, ongoing *jobs, out progress.Output, start time.Time) error {
	actives, err := p.Store.ListStatuses(ctx, "")
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		logrus.WithError(err).Error("status check failed")
		return nil
	}
	pulling := make(map[string]content.Status, len(actives))

	// update status of status entries!
	for _, status := range actives {
		pulling[status.Ref] = status
	}

	for _, j := range ongoing.Jobs() {
		key := remotes.MakeRefKey(ctx, j)
		if info, ok := pulling[key]; ok {
			out.WriteProgress(progress.Progress{
				ID:      stringid.TruncateID(j.Digest.Encoded()),
				Action:  "Downloading",
				Current: info.Offset,
				Total:   info.Total,
			})
			continue
		}

		info, err := p.Store.Info(ctx, j.Digest)
		if err != nil {
			if !cerrdefs.IsNotFound(err) {
				return err
			}
		} else if info.CreatedAt.After(start) {
			out.WriteProgress(progress.Progress{
				ID:         stringid.TruncateID(j.Digest.Encoded()),
				Action:     "Download complete",
				HideCounts: true,
				LastUpdate: true,
			})
			ongoing.Remove(j)
		} else if p.ShowExists {
			out.WriteProgress(progress.Progress{
				ID:         stringid.TruncateID(j.Digest.Encoded()),
				Action:     "Exists",
				HideCounts: true,
				LastUpdate: true,
			})
			ongoing.Remove(j)
		}
	}
	return nil
}

type pushProgress struct {
	Tracker   docker.StatusTracker
	mountable map[digest.Digest]struct{}
	mutex     sync.Mutex
}

func (p *pushProgress) addMountable(dgst digest.Digest) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.mountable == nil {
		p.mountable = map[digest.Digest]struct{}{}
	}
	p.mountable[dgst] = struct{}{}
}

func (p *pushProgress) isMountable(dgst digest.Digest) bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.mountable == nil {
		return false
	}
	_, has := p.mountable[dgst]
	return has
}

func (p *pushProgress) UpdateProgress(ctx context.Context, ongoing *jobs, out progress.Output, start time.Time) error {
	for _, j := range ongoing.Jobs() {
		key := remotes.MakeRefKey(ctx, j)
		id := stringid.TruncateID(j.Digest.Encoded())

		status, err := p.Tracker.GetStatus(key)
		if err != nil {
			if cerrdefs.IsNotFound(err) {
				progress.Update(out, id, "Waiting")
				continue
			}
		}

		if status.Committed && status.Offset >= status.Total {
			if p.isMountable(j.Digest) {
				progress.Update(out, id, "Mounted")
			} else {
				progress.Update(out, id, "Pushed")
			}
			ongoing.Remove(j)
			continue
		}

		out.WriteProgress(progress.Progress{
			ID:      id,
			Action:  "Pushing",
			Current: status.Offset,
			Total:   status.Total,
		})
	}

	return nil
}

type combinedProgress []progressUpdater

func (combined combinedProgress) UpdateProgress(ctx context.Context, ongoing *jobs, out progress.Output, start time.Time) error {
	for _, p := range combined {
		err := p.UpdateProgress(ctx, ongoing, out, start)
		if err != nil {
			return err
		}
	}
	return nil
}
