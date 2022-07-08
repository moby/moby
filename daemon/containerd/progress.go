package containerd

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/remotes"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
)

func showProgress(ctx context.Context, ongoing *jobs, cs content.Store, w io.Writer, stop chan struct{}) {
	var (
		out    = streamformatter.NewJSONProgressOutput(w, false)
		ticker = time.NewTicker(100 * time.Millisecond)
		start  = time.Now()
		done   bool
	)
	defer ticker.Stop()

outer:
	for {
		select {
		case <-ticker.C:
			if !ongoing.IsResolved() {
				continue
			}

			pulling := map[string]content.Status{}
			if !done {
				actives, err := cs.ListStatuses(ctx, "")
				if err != nil {
					log.G(ctx).WithError(err).Error("status check failed")
					continue
				}
				// update status of status entries!
				for _, status := range actives {
					pulling[status.Ref] = status
				}
			}

			// update inactive jobs
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
					if !errdefs.IsNotFound(err) {
						log.G(ctx).WithError(err).Error("failed to get content info")
						continue outer
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
			if done {
				return
			}
		case <-stop:
			done = true // allow ui to update once more
		case <-ctx.Done():
			return
		}
	}
}

// jobs holds a list of layers being downloaded to pull reference set by name
type jobs struct {
	name     string
	resolved bool // resolved is set to true once remote image metadata has been downloaded from registry
	descs    map[digest.Digest]v1.Descriptor
	mu       sync.Mutex
}

// newJobs creates a new instance of the job status tracker
func newJobs() *jobs {
	return &jobs{
		descs: map[digest.Digest]v1.Descriptor{},
	}
}

// IsResolved checks whether a descriptor has been resolved
func (j *jobs) IsResolved() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.resolved
}

// Add adds a descriptor to be tracked
func (j *jobs) Add(desc v1.Descriptor) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if _, ok := j.descs[desc.Digest]; ok {
		return
	}
	j.descs[desc.Digest] = desc
	j.resolved = true
}

// Remove removes a descriptor
func (j *jobs) Remove(desc v1.Descriptor) {
	j.mu.Lock()
	defer j.mu.Unlock()

	delete(j.descs, desc.Digest)
}

// Jobs returns a list of all tracked descriptors
func (j *jobs) Jobs() []v1.Descriptor {
	j.mu.Lock()
	defer j.mu.Unlock()

	descs := make([]v1.Descriptor, 0, len(j.descs))
	for _, d := range j.descs {
		descs = append(descs, d)
	}
	return descs
}
