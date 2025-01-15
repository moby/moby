package containerd

import (
	"context"
	"testing"

	c8dimages "github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/metadata"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/log/logtest"
	"github.com/docker/docker/container"
	daemonevents "github.com/docker/docker/daemon/events"
	dimages "github.com/docker/docker/daemon/images"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageDelete(t *testing.T) {
	ctx := namespaces.WithNamespace(context.TODO(), "testing")

	for _, tc := range []struct {
		ref       string
		starting  []c8dimages.Image
		remaining []c8dimages.Image
		err       error
		// TODO: Records
		// TODO: Containers
		// TODO: Events
	}{
		{
			ref: "nothingthere",
			err: dimages.ErrImageDoesNotExist{Ref: nameTag("nothingthere", "latest")},
		},
		{
			ref: "justoneimage",
			starting: []c8dimages.Image{
				{
					Name:   "docker.io/library/justoneimage:latest",
					Target: desc(10),
				},
			},
		},
		{
			ref: "justoneref",
			starting: []c8dimages.Image{
				{
					Name:   "docker.io/library/justoneref:latest",
					Target: desc(10),
				},
				{
					Name:   "docker.io/library/differentrepo:latest",
					Target: desc(10),
				},
			},
			remaining: []c8dimages.Image{
				{
					Name:   "docker.io/library/differentrepo:latest",
					Target: desc(10),
				},
			},
		},
		{
			ref: "hasdigest",
			starting: []c8dimages.Image{
				{
					Name:   "docker.io/library/hasdigest:latest",
					Target: desc(10),
				},
				{
					Name:   "docker.io/library/hasdigest@" + digestFor(10).String(),
					Target: desc(10),
				},
			},
		},
		{
			ref: digestFor(11).String(),
			starting: []c8dimages.Image{
				{
					Name:   "docker.io/library/byid:latest",
					Target: desc(11),
				},
				{
					Name:   "docker.io/library/byid@" + digestFor(11).String(),
					Target: desc(11),
				},
			},
		},
		{
			ref: "bydigest@" + digestFor(12).String(),
			starting: []c8dimages.Image{
				{
					Name:   "docker.io/library/bydigest:latest",
					Target: desc(12),
				},
				{
					Name:   "docker.io/library/bydigest@" + digestFor(12).String(),
					Target: desc(12),
				},
			},
		},
		{
			ref: "onerefoftwo",
			starting: []c8dimages.Image{
				{
					Name:   "docker.io/library/onerefoftwo:latest",
					Target: desc(12),
				},
				{
					Name:   "docker.io/library/onerefoftwo:other",
					Target: desc(12),
				},
				{
					Name:   "docker.io/library/onerefoftwo@" + digestFor(12).String(),
					Target: desc(12),
				},
			},
			remaining: []c8dimages.Image{
				{
					Name:   "docker.io/library/onerefoftwo:other",
					Target: desc(12),
				},
				{
					Name:   "docker.io/library/onerefoftwo@" + digestFor(12).String(),
					Target: desc(12),
				},
			},
		},
		{
			ref: "otherreporemaining",
			starting: []c8dimages.Image{
				{
					Name:   "docker.io/library/otherreporemaining:latest",
					Target: desc(12),
				},
				{
					Name:   "docker.io/library/otherreporemaining@" + digestFor(12).String(),
					Target: desc(12),
				},
				{
					Name:   "docker.io/library/someotherrepo:latest",
					Target: desc(12),
				},
			},
			remaining: []c8dimages.Image{
				{
					Name:   "docker.io/library/someotherrepo:latest",
					Target: desc(12),
				},
			},
		},
		{
			ref: "repoanddigest@" + digestFor(15).String(),
			starting: []c8dimages.Image{
				{
					Name:   "docker.io/library/repoanddigest:latest",
					Target: desc(15),
				},
				{
					Name:   "docker.io/library/repoanddigest:latest@" + digestFor(15).String(),
					Target: desc(15),
				},
				{
					Name:   "docker.io/library/someotherrepo:latest",
					Target: desc(15),
				},
			},
			remaining: []c8dimages.Image{
				{
					Name:   "docker.io/library/someotherrepo:latest",
					Target: desc(15),
				},
			},
		},
		{
			ref: "repoanddigestothertags@" + digestFor(15).String(),
			starting: []c8dimages.Image{
				{
					Name:   "docker.io/library/repoanddigestothertags:v1",
					Target: desc(15),
				},
				{
					Name:   "docker.io/library/repoanddigestothertags:v1@" + digestFor(15).String(),
					Target: desc(15),
				},
				{
					Name:   "docker.io/library/repoanddigestothertags:v2",
					Target: desc(15),
				},
				{
					Name:   "docker.io/library/repoanddigestothertags:v2@" + digestFor(15).String(),
					Target: desc(15),
				},
				{
					Name:   "docker.io/library/someotherrepo:latest",
					Target: desc(15),
				},
			},
			remaining: []c8dimages.Image{
				{
					Name:   "docker.io/library/someotherrepo:latest",
					Target: desc(15),
				},
			},
		},
		{
			ref: "repoanddigestzerocase@" + digestFor(16).String(),
			starting: []c8dimages.Image{
				{
					Name:   "docker.io/library/someotherrepo:latest",
					Target: desc(16),
				},
			},
			remaining: []c8dimages.Image{
				{
					Name:   "docker.io/library/someotherrepo:latest",
					Target: desc(16),
				},
			},
			err: dimages.ErrImageDoesNotExist{Ref: nameDigest("repoanddigestzerocase", digestFor(16))},
		},
	} {
		t.Run(tc.ref, func(t *testing.T) {
			t.Parallel()
			ctx := logtest.WithT(ctx, t)
			mdb := newTestDB(ctx, t)
			service := &ImageService{
				images:        metadata.NewImageStore(mdb),
				containers:    emptyTestContainerStore(),
				eventsService: daemonevents.New(),
			}
			for _, img := range tc.starting {
				if _, err := service.images.Create(ctx, img); err != nil {
					t.Fatalf("failed to create image %q: %v", img.Name, err)
				}
			}

			_, err := service.ImageDelete(ctx, tc.ref, false, false)
			if tc.err == nil {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tc.err.Error())
			}

			all, err := service.images.List(ctx)
			assert.NilError(t, err)
			assert.Assert(t, is.Len(tc.remaining, len(all)))

			// Order should match
			for i := range all {
				assert.Check(t, is.Equal(all[i].Name, tc.remaining[i].Name), "image[%d]", i)
				assert.Check(t, is.Equal(all[i].Target.Digest, tc.remaining[i].Target.Digest), "image[%d]", i)
				// TODO: Check labels too
			}
		})
	}
}

type testContainerStore struct{}

func emptyTestContainerStore() container.Store {
	return &testContainerStore{}
}

func (*testContainerStore) Add(string, *container.Container) {}

func (*testContainerStore) Get(string) *container.Container {
	return nil
}

func (*testContainerStore) Delete(string) {}

func (*testContainerStore) List() []*container.Container {
	return []*container.Container{}
}

func (*testContainerStore) Size() int {
	return 0
}

func (*testContainerStore) First(container.StoreFilter) *container.Container {
	return nil
}

func (*testContainerStore) ApplyAll(container.StoreReducer) {}
