package containerd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/log/logtest"
	imagetypes "github.com/docker/docker/api/types/image"
	daemonevents "github.com/docker/docker/daemon/events"
	"github.com/docker/docker/internal/testutils/specialimage"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func imagesFromIndex(index ...*ocispec.Index) []images.Image {
	var imgs []images.Image
	for _, idx := range index {
		for _, desc := range idx.Manifests {
			imgs = append(imgs, images.Image{
				Name:   desc.Annotations["io.containerd.image.name"],
				Target: desc,
			})
		}
	}
	return imgs
}

func TestImageList(t *testing.T) {
	ctx := namespaces.WithNamespace(context.TODO(), "testing")

	blobsDir := t.TempDir()

	multilayer, err := specialimage.MultiLayer(blobsDir)
	assert.NilError(t, err)

	twoplatform, err := specialimage.TwoPlatform(blobsDir)
	assert.NilError(t, err)

	cs := &blobsDirContentStore{blobs: filepath.Join(blobsDir, "blobs/sha256")}

	snapshotter := &testSnapshotterService{}

	for _, tc := range []struct {
		name   string
		images []images.Image
		opts   imagetypes.ListOptions

		check func(*testing.T, []*imagetypes.Summary) // Change the type of the check function
	}{
		{
			name:   "one multi-layer image",
			images: imagesFromIndex(multilayer),
			check: func(t *testing.T, all []*imagetypes.Summary) { // Change the type of the check function
				assert.Check(t, is.Len(all, 1))

				assert.Check(t, is.Equal(all[0].ID, multilayer.Manifests[0].Digest.String()))
				assert.Check(t, is.DeepEqual(all[0].RepoTags, []string{"multilayer:latest"}))
			},
		},
		{
			name:   "one image with two platforms is still one entry",
			images: imagesFromIndex(twoplatform),
			check: func(t *testing.T, all []*imagetypes.Summary) { // Change the type of the check function
				assert.Check(t, is.Len(all, 1))

				assert.Check(t, is.Equal(all[0].ID, twoplatform.Manifests[0].Digest.String()))
				assert.Check(t, is.DeepEqual(all[0].RepoTags, []string{"twoplatform:latest"}))
			},
		},
		{
			name:   "two images are two entries",
			images: imagesFromIndex(multilayer, twoplatform),
			check: func(t *testing.T, all []*imagetypes.Summary) { // Change the type of the check function
				assert.Check(t, is.Len(all, 2))

				assert.Check(t, is.Equal(all[0].ID, multilayer.Manifests[0].Digest.String()))
				assert.Check(t, is.DeepEqual(all[0].RepoTags, []string{"multilayer:latest"}))

				assert.Check(t, is.Equal(all[1].ID, twoplatform.Manifests[0].Digest.String()))
				assert.Check(t, is.DeepEqual(all[1].RepoTags, []string{"twoplatform:latest"}))
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := logtest.WithT(ctx, t)
			mdb := newTestDB(ctx, t)

			snapshotters := map[string]snapshots.Snapshotter{
				containerd.DefaultSnapshotter: snapshotter,
			}

			service := &ImageService{
				images:              metadata.NewImageStore(mdb),
				containers:          emptyTestContainerStore(),
				content:             cs,
				eventsService:       daemonevents.New(),
				snapshotterServices: snapshotters,
				snapshotter:         containerd.DefaultSnapshotter,
			}

			// containerd.Image gets the services directly from containerd.Client
			// so we need to create a "fake" containerd.Client with the test services.
			c8dCli, err := containerd.New("", containerd.WithServices(
				containerd.WithImageStore(service.images),
				containerd.WithContentStore(cs),
				containerd.WithSnapshotters(snapshotters),
			))
			assert.NilError(t, err)

			service.client = c8dCli

			for _, img := range tc.images {
				_, err := service.images.Create(ctx, img)
				assert.NilError(t, err)
			}

			all, err := service.Images(ctx, tc.opts)
			assert.NilError(t, err)

			sort.Slice(all, func(i, j int) bool {
				firstTag := func(idx int) string {
					if len(all[idx].RepoTags) > 0 {
						return all[idx].RepoTags[0]
					}
					return ""
				}
				return firstTag(i) < firstTag(j)
			})

			tc.check(t, all)
		})
	}

}

type blobsDirContentStore struct {
	blobs string
}

type fileReaderAt struct {
	*os.File
}

func (f *fileReaderAt) Size() int64 {
	fi, err := f.Stat()
	if err != nil {
		return -1
	}
	return fi.Size()
}

func (s *blobsDirContentStore) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	p := filepath.Join(s.blobs, desc.Digest.Encoded())
	r, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	return &fileReaderAt{r}, nil
}

func (s *blobsDirContentStore) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	return nil, fmt.Errorf("read-only")
}

func (s *blobsDirContentStore) Status(ctx context.Context, _ string) (content.Status, error) {
	return content.Status{}, fmt.Errorf("not implemented")
}

func (s *blobsDirContentStore) Delete(ctx context.Context, dgst digest.Digest) error {
	return fmt.Errorf("read-only")
}

func (s *blobsDirContentStore) ListStatuses(ctx context.Context, filters ...string) ([]content.Status, error) {
	return nil, nil
}

func (s *blobsDirContentStore) Abort(ctx context.Context, ref string) error {
	return fmt.Errorf("not implemented")
}

func (s *blobsDirContentStore) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	entries, err := os.ReadDir(s.blobs)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		d := digest.FromString(e.Name())
		if d == "" {
			continue
		}

		stat, err := e.Info()
		if err != nil {
			return err
		}

		if err := fn(content.Info{Digest: d, Size: stat.Size()}); err != nil {
			return err
		}
	}

	return nil
}

func (s *blobsDirContentStore) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	f, err := os.Open(filepath.Join(s.blobs, dgst.Encoded()))
	if err != nil {
		return content.Info{}, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return content.Info{}, err
	}

	return content.Info{
		Digest: dgst,
		Size:   stat.Size(),
	}, nil
}

func (s *blobsDirContentStore) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	return content.Info{}, fmt.Errorf("read-only")
}
