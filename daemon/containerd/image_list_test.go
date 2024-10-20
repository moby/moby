package containerd

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/snapshots"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/containerd/log/logtest"
	"github.com/containerd/platforms"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/container"
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

func BenchmarkImageList(b *testing.B) {
	populateStore := func(ctx context.Context, is *ImageService, dir string,
		count int,
		// % chance for each image to spawn containers
		containerChance int,
		// Maximum container count if the image is decided to spawn containers (chance above)
		maxContainerCount int,
	) {
		// Use constant seed for reproducibility
		src := rand.NewSource(1982731263716)

		for i := 0; i < count; i++ {
			platform := platforms.DefaultSpec()

			// 20% is other architecture than the host
			if i%5 == 0 {
				platform.Architecture = "other"
			}

			idx, err := specialimage.RandomSinglePlatform(dir, platform, src)
			assert.NilError(b, err)

			r1 := int(src.Int63())
			r2 := int(src.Int63())

			imgs := imagesFromIndex(idx)
			for _, desc := range imgs {
				_, err := is.images.Create(ctx, desc)
				assert.NilError(b, err)

				if r1%100 >= containerChance {
					continue
				}

				containersCount := r2 % maxContainerCount
				for j := 0; j < containersCount; j++ {
					id := digest.FromString(desc.Name + strconv.Itoa(i)).String()

					target := desc.Target
					is.containers.Add(id, &container.Container{
						ID:            id,
						ImageManifest: &target,
					})
				}
			}
		}
	}

	for _, count := range []int{10, 100, 1000} {
		count := count
		csDir := b.TempDir()

		ctx := namespaces.WithNamespace(context.TODO(), "testing-"+strconv.Itoa(count))

		cs := &delayedStore{
			store:    &blobsDirContentStore{blobs: filepath.Join(csDir, "blobs/sha256")},
			overhead: 500 * time.Microsecond,
		}

		is := fakeImageService(b, ctx, cs)

		// Every generated image has a 10% chance to spawn up to 5 containers
		const containerChance = 10
		const maxContainerCount = 5
		populateStore(ctx, is, csDir, count, containerChance, maxContainerCount)

		b.Run(strconv.Itoa(count)+"-images", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := is.Images(ctx, imagetypes.ListOptions{All: true})
				assert.NilError(b, err)
			}
		})
	}
}

func TestImageListCheckTotalSize(t *testing.T) {
	ctx := namespaces.WithNamespace(context.TODO(), "testing")

	blobsDir := t.TempDir()
	cs := &blobsDirContentStore{blobs: filepath.Join(blobsDir, "blobs/sha256")}

	twoplatform, mfstsDescs, err := specialimage.MultiPlatform(blobsDir, "test:latest", []ocispec.Platform{
		{OS: "linux", Architecture: "arm64"},
		{OS: "linux", Architecture: "amd64"},
	})
	assert.NilError(t, err)

	ctx = logtest.WithT(ctx, t)
	service := fakeImageService(t, ctx, cs)

	_, err = service.images.Create(ctx, imagesFromIndex(twoplatform)[0])
	assert.NilError(t, err)

	all, err := service.Images(ctx, imagetypes.ListOptions{Manifests: true})
	assert.NilError(t, err)

	assert.Check(t, is.Len(all, 1))
	assert.Check(t, is.Len(all[0].Manifests, 2))

	// TODO: The test snapshotter doesn't do anything, so the size is always 0.
	assert.Check(t, is.Equal(all[0].Manifests[0].ImageData.Size.Unpacked, int64(0)))
	assert.Check(t, is.Equal(all[0].Manifests[1].ImageData.Size.Unpacked, int64(0)))

	mfstArm64 := mfstsDescs[0]
	mfstAmd64 := mfstsDescs[1]

	indexSize := blobSize(t, ctx, cs, twoplatform.Manifests[0].Digest)
	arm64ManifestSize := blobSize(t, ctx, cs, mfstArm64.Digest)
	amd64ManifestSize := blobSize(t, ctx, cs, mfstAmd64.Digest)

	var arm64Mfst, amd64Mfst ocispec.Manifest
	assert.NilError(t, readJSON(ctx, cs, mfstArm64, &arm64Mfst))
	assert.NilError(t, readJSON(ctx, cs, mfstAmd64, &amd64Mfst))

	// MultiPlatform should produce a single layer. If these fail, the test needs to be adjusted.
	assert.Assert(t, is.Len(arm64Mfst.Layers, 1))
	assert.Assert(t, is.Len(amd64Mfst.Layers, 1))

	arm64ConfigSize := blobSize(t, ctx, cs, arm64Mfst.Config.Digest)
	amd64ConfigSize := blobSize(t, ctx, cs, amd64Mfst.Config.Digest)

	arm64LayerSize := blobSize(t, ctx, cs, arm64Mfst.Layers[0].Digest)
	amd64LayerSize := blobSize(t, ctx, cs, amd64Mfst.Layers[0].Digest)

	allTotalSize := indexSize +
		arm64ManifestSize + amd64ManifestSize +
		arm64ConfigSize + amd64ConfigSize +
		arm64LayerSize + amd64LayerSize

	assert.Check(t, is.Equal(all[0].Size, allTotalSize-indexSize))

	assert.Check(t, is.Equal(all[0].Manifests[0].Size.Content, arm64ManifestSize+arm64ConfigSize+arm64LayerSize))
	assert.Check(t, is.Equal(all[0].Manifests[1].Size.Content, amd64ManifestSize+amd64ConfigSize+amd64LayerSize))

	// TODO: This should also include the Size.Unpacked, but the test snapshotter doesn't do anything yet
	assert.Check(t, is.Equal(all[0].Manifests[0].Size.Total, amd64ManifestSize+amd64ConfigSize+amd64LayerSize))
	assert.Check(t, is.Equal(all[0].Manifests[1].Size.Total, amd64ManifestSize+amd64ConfigSize+amd64LayerSize))
}

func blobSize(t *testing.T, ctx context.Context, cs content.Store, dgst digest.Digest) int64 {
	info, err := cs.Info(ctx, dgst)
	assert.NilError(t, err)
	return info.Size
}

func TestImageList(t *testing.T) {
	ctx := namespaces.WithNamespace(context.TODO(), "testing")

	blobsDir := t.TempDir()

	multilayer, err := specialimage.MultiLayer(blobsDir)
	assert.NilError(t, err)

	twoplatform, err := specialimage.TwoPlatform(blobsDir)
	assert.NilError(t, err)

	emptyIndex, err := specialimage.EmptyIndex(blobsDir)
	assert.NilError(t, err)

	configTarget, err := specialimage.ConfigTarget(blobsDir)
	assert.NilError(t, err)

	textplain, err := specialimage.TextPlain(blobsDir)
	assert.NilError(t, err)

	cs := &blobsDirContentStore{blobs: filepath.Join(blobsDir, "blobs/sha256")}

	for _, tc := range []struct {
		name   string
		images []images.Image
		opts   imagetypes.ListOptions

		check func(*testing.T, []*imagetypes.Summary)
	}{
		{
			name:   "one multi-layer image",
			images: imagesFromIndex(multilayer),
			check: func(t *testing.T, all []*imagetypes.Summary) {
				assert.Check(t, is.Len(all, 1))

				assert.Check(t, is.Equal(all[0].ID, multilayer.Manifests[0].Digest.String()))
				assert.Check(t, is.DeepEqual(all[0].RepoTags, []string{"multilayer:latest"}))

				assert.Check(t, is.Len(all[0].Manifests, 1))
				assert.Check(t, all[0].Manifests[0].Available)
				assert.Check(t, is.Equal(all[0].Manifests[0].Kind, imagetypes.ManifestKindImage))
			},
		},
		{
			name:   "one image with two platforms is still one entry",
			images: imagesFromIndex(twoplatform),
			check: func(t *testing.T, all []*imagetypes.Summary) {
				assert.Check(t, is.Len(all, 1))

				assert.Check(t, is.Equal(all[0].ID, twoplatform.Manifests[0].Digest.String()))
				assert.Check(t, is.DeepEqual(all[0].RepoTags, []string{"twoplatform:latest"}))

				i := all[0]
				assert.Check(t, is.Len(i.Manifests, 2))

				assert.Check(t, is.Equal(i.Manifests[0].Kind, imagetypes.ManifestKindImage))
				if assert.Check(t, i.Manifests[0].ImageData != nil) {
					assert.Check(t, is.Equal(i.Manifests[0].ImageData.Platform.Architecture, "amd64"))
				}
				assert.Check(t, is.Equal(i.Manifests[1].Kind, imagetypes.ManifestKindImage))
				if assert.Check(t, i.Manifests[1].ImageData != nil) {
					assert.Check(t, is.Equal(i.Manifests[1].ImageData.Platform.Architecture, "arm64"))
				}
			},
		},
		{
			name:   "two images are two entries",
			images: imagesFromIndex(multilayer, twoplatform),
			check: func(t *testing.T, all []*imagetypes.Summary) {
				assert.Check(t, is.Len(all, 2))

				assert.Check(t, is.Equal(all[0].ID, multilayer.Manifests[0].Digest.String()))
				assert.Check(t, is.DeepEqual(all[0].RepoTags, []string{"multilayer:latest"}))

				assert.Check(t, is.Equal(all[1].ID, twoplatform.Manifests[0].Digest.String()))
				assert.Check(t, is.DeepEqual(all[1].RepoTags, []string{"twoplatform:latest"}))

				assert.Check(t, is.Len(all[0].Manifests, 1))
				assert.Check(t, is.Len(all[1].Manifests, 2))

				assert.Check(t, is.Equal(all[0].Manifests[0].Kind, imagetypes.ManifestKindImage))

				assert.Check(t, is.Equal(all[1].Manifests[0].Kind, imagetypes.ManifestKindImage))
				assert.Check(t, is.Equal(all[1].Manifests[1].Kind, imagetypes.ManifestKindImage))
			},
		},
		{
			name:   "three images, one is an empty index",
			images: imagesFromIndex(multilayer, emptyIndex, twoplatform),
			check: func(t *testing.T, all []*imagetypes.Summary) {
				assert.Check(t, is.Len(all, 3))
			},
		},
		{
			name:   "one good image, second has config as a target",
			images: imagesFromIndex(multilayer, configTarget),
			check: func(t *testing.T, all []*imagetypes.Summary) {
				assert.Check(t, is.Len(all, 2))

				sort.Slice(all, func(i, j int) bool {
					return slices.Contains(all[i].RepoTags, "multilayer:latest")
				})

				assert.Check(t, is.Equal(all[0].ID, multilayer.Manifests[0].Digest.String()))
				assert.Check(t, is.Len(all[0].Manifests, 1))

				assert.Check(t, is.Equal(all[1].ID, configTarget.Manifests[0].Digest.String()))
				assert.Check(t, is.Len(all[1].Manifests, 0))
			},
		},
		{
			name:   "a non-container image manifest",
			images: imagesFromIndex(textplain),
			check: func(t *testing.T, all []*imagetypes.Summary) {
				assert.Check(t, is.Len(all, 1))
				assert.Check(t, is.Equal(all[0].ID, textplain.Manifests[0].Digest.String()))

				assert.Assert(t, is.Len(all[0].Manifests, 0))
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := logtest.WithT(ctx, t)
			service := fakeImageService(t, ctx, cs)

			for _, img := range tc.images {
				_, err := service.images.Create(ctx, img)
				assert.NilError(t, err)
			}

			opts := tc.opts
			opts.Manifests = true
			all, err := service.Images(ctx, opts)
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

func fakeImageService(t testing.TB, ctx context.Context, cs content.Store) *ImageService {
	snapshotter := &testSnapshotterService{}

	mdb := newTestDB(ctx, t)

	snapshotters := map[string]snapshots.Snapshotter{
		containerd.DefaultSnapshotter: snapshotter,
	}

	service := &ImageService{
		images:              metadata.NewImageStore(mdb),
		containers:          container.NewMemoryStore(),
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
	return service
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
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", cerrdefs.ErrNotFound, desc.Digest)
		}
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
	p := filepath.Join(s.blobs, dgst.Encoded())
	return os.Remove(p)
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
		if errors.Is(err, os.ErrNotExist) {
			return content.Info{}, fmt.Errorf("%w: %s", cerrdefs.ErrNotFound, dgst)
		}
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

// delayedStore is a content store wrapper that adds a constant delay to all
// operations in order to imitate gRPC overhead.
//
// The delay is constant to make the benchmark results more reproducible
// Since content store may be accessed concurrently random delay would be
// order-dependent.
type delayedStore struct {
	store    content.Store
	overhead time.Duration
}

func (s *delayedStore) delay() {
	time.Sleep(s.overhead)
}

func (s *delayedStore) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	s.delay()
	return s.store.ReaderAt(ctx, desc)
}

func (s *delayedStore) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	s.delay()
	return s.store.Writer(ctx, opts...)
}

func (s *delayedStore) Status(ctx context.Context, st string) (content.Status, error) {
	s.delay()
	return s.store.Status(ctx, st)
}

func (s *delayedStore) Delete(ctx context.Context, dgst digest.Digest) error {
	s.delay()
	return s.store.Delete(ctx, dgst)
}

func (s *delayedStore) ListStatuses(ctx context.Context, filters ...string) ([]content.Status, error) {
	s.delay()
	return s.store.ListStatuses(ctx, filters...)
}

func (s *delayedStore) Abort(ctx context.Context, ref string) error {
	s.delay()
	return s.store.Abort(ctx, ref)
}

func (s *delayedStore) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	s.delay()
	return s.store.Walk(ctx, fn, filters...)
}

func (s *delayedStore) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	s.delay()
	return s.store.Info(ctx, dgst)
}

func (s *delayedStore) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	s.delay()
	return s.store.Update(ctx, info, fieldpaths...)
}
