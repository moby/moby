package containerd

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	cerrdefs "github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

type updateProgressFunc func(ctx context.Context, ongoing *jobs, output progress.Output, start time.Time) error

func showProgress(ctx context.Context, ongoing *jobs, w io.Writer, updateFunc updateProgressFunc) func() {
	stop := make(chan struct{})
	ctx, cancelProgress := context.WithCancel(ctx)

	var (
		out    = streamformatter.NewJSONProgressOutput(w, false)
		ticker = time.NewTicker(100 * time.Millisecond)
		start  = time.Now()
		done   bool
	)

	for _, j := range ongoing.Jobs() {
		id := stringid.TruncateID(j.Digest.Encoded())
		progress.Update(out, id, "Preparing")
	}

	go func() {
		defer func() {
			ticker.Stop()
			stop <- struct{}{}
		}()

		for {
			select {
			case <-ticker.C:
				if !ongoing.IsResolved() {
					continue
				}
				err := updateFunc(ctx, ongoing, out, start)
				if err != nil {
					logrus.WithError(err).Error("Updating progress failed")
					return
				}

				if done {
					return
				}
			case <-ctx.Done():
				done = true
			}
		}
	}()

	return func() {
		cancelProgress()
		<-stop
	}
}

func pushProgress(tracker docker.StatusTracker) updateProgressFunc {
	return func(ctx context.Context, ongoing *jobs, out progress.Output, start time.Time) error {
		for _, j := range ongoing.Jobs() {
			key := remotes.MakeRefKey(ctx, j)
			id := stringid.TruncateID(j.Digest.Encoded())

			status, err := tracker.GetStatus(key)
			if err != nil {
				if cerrdefs.IsNotFound(err) {
					progress.Update(out, id, "Waiting")
					continue
				} else {
					return err
				}

			}

			logrus.WithField("status", status).WithField("id", id).Debug("Status update")

			if status.Committed && status.Offset >= status.Total {
				progress.Update(out, id, "Pushed")
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
}

func pullProgress(cs content.Store) updateProgressFunc {
	return func(ctx context.Context, ongoing *jobs, out progress.Output, start time.Time) error {
		pulling := map[string]content.Status{}
		actives, err := cs.ListStatuses(ctx, "")
		if err != nil {
			log.G(ctx).WithError(err).Error("status check failed")
			return nil
		}
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

			info, err := cs.Info(ctx, j.Digest)
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
			} else {
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
}

type jobs struct {
	resolved bool // resolved is set to true once all jobs are added
	descs    map[digest.Digest]ocispec.Descriptor
	mu       sync.Mutex
}

// newJobs creates a new instance of the job status tracker
func newJobs() *jobs {
	return &jobs{
		descs: map[digest.Digest]ocispec.Descriptor{},
	}
}

// IsResolved checks whether a descriptor has been resolved
func (j *jobs) IsResolved() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.resolved
}

// Add adds a descriptor to be tracked
func (j *jobs) Add(desc ocispec.Descriptor) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if _, ok := j.descs[desc.Digest]; ok {
		return
	}
	j.descs[desc.Digest] = desc
	j.resolved = true
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
