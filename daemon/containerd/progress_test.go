package containerd

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/containerd/containerd/v2/core/content"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/v2/daemon/internal/progress"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
)

type fakeContentStore struct {
	content.Store
}

func (fakeContentStore) ListStatuses(context.Context, ...string) ([]content.Status, error) {
	return nil, nil
}

func (fakeContentStore) Info(context.Context, digest.Digest) (content.Info, error) {
	return content.Info{}, cerrdefs.ErrNotFound
}

func TestPullProgressHideLayersConcurrent(t *testing.T) {
	j := newJobs()
	j.Add(ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayer,
		Digest:    digest.FromString("layer"),
	})

	pp := &pullProgress{
		store: fakeContentStore{},
	}

	var wg sync.WaitGroup
	wg.Go(func() {
		for range 1000 {
			err := pp.UpdateProgress(t.Context(), j, progress.DiscardOutput(), time.Now())
			assert.NilError(t, err)
		}
	})
	wg.Go(func() {
		for range 1000 {
			pp.hideLayers.Store(true)
		}
	})
	wg.Wait()
}
