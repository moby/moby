package containerd

import (
	"context"
	"io"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/log/logtest"
	"github.com/distribution/reference"
	dockerimages "github.com/docker/docker/daemon/images"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"go.etcd.io/bbolt"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestLookup(t *testing.T) {
	ctx := namespaces.WithNamespace(context.TODO(), "testing")
	ctx = logtest.WithT(ctx, t)
	mdb := newTestDB(ctx, t)
	service := &ImageService{
		images: metadata.NewImageStore(mdb),
	}

	ubuntuLatest := images.Image{
		Name:   "docker.io/library/ubuntu:latest",
		Target: desc(10),
	}
	ubuntuLatestWithDigest := images.Image{
		Name:   "docker.io/library/ubuntu:latest@" + digestFor(10).String(),
		Target: desc(10),
	}
	ubuntuLatestWithOldDigest := images.Image{
		Name:   "docker.io/library/ubuntu:latest@" + digestFor(11).String(),
		Target: desc(11),
	}
	ambiguousShortName := images.Image{
		Name:   "docker.io/library/abcdef:latest",
		Target: desc(12),
	}
	ambiguousShortNameWithDigest := images.Image{
		Name:   "docker.io/library/abcdef:latest@" + digestFor(12).String(),
		Target: desc(12),
	}
	shortNameIsHashAlgorithm := images.Image{
		Name:   "docker.io/library/sha256:defcab",
		Target: desc(13),
	}

	testImages := []images.Image{
		ubuntuLatest,
		ubuntuLatestWithDigest,
		ubuntuLatestWithOldDigest,
		ambiguousShortName,
		ambiguousShortNameWithDigest,
		shortNameIsHashAlgorithm,
		{
			Name:   "docker.io/test/volatile:retried",
			Target: desc(14),
		},
		{
			Name:   "docker.io/test/volatile:inconsistent",
			Target: desc(15),
		},
	}
	for _, img := range testImages {
		if _, err := service.images.Create(ctx, img); err != nil {
			t.Fatalf("failed to create image %q: %v", img.Name, err)
		}
	}

	for _, tc := range []struct {
		lookup string
		img    *images.Image
		all    []images.Image
		err    error
	}{
		{
			// Get ubuntu images with default "latest" tag
			lookup: "ubuntu",
			img:    &ubuntuLatest,
			all:    []images.Image{ubuntuLatest, ubuntuLatestWithDigest},
		},
		{
			// Get all images by image id
			lookup: ubuntuLatest.Target.Digest.String(),
			img:    nil,
			all:    []images.Image{ubuntuLatest, ubuntuLatestWithDigest},
		},
		{
			// Fail to lookup reference with no tag, reference has both tag and digest
			lookup: "ubuntu@" + ubuntuLatestWithOldDigest.Target.Digest.String(),
			img:    nil,
			all:    []images.Image{ubuntuLatestWithOldDigest},
		},
		{
			// Get all image with both tag and digest
			lookup: "ubuntu:latest@" + ubuntuLatestWithOldDigest.Target.Digest.String(),
			img:    &ubuntuLatestWithOldDigest,
			all:    []images.Image{ubuntuLatestWithOldDigest},
		},
		{
			// Fail to lookup reference with no tag for digest that doesn't exist
			lookup: "ubuntu@" + digestFor(20).String(),
			err:    dockerimages.ErrImageDoesNotExist{Ref: nameDigest("ubuntu", digestFor(20))},
		},
		{
			// Fail to lookup reference with nonexistent tag
			lookup: "ubuntu:nonexistent",
			err:    dockerimages.ErrImageDoesNotExist{Ref: nameTag("ubuntu", "nonexistent")},
		},
		{
			// Get abcdef image which also matches short image id
			lookup: "abcdef",
			img:    &ambiguousShortName,
			all:    []images.Image{ambiguousShortName, ambiguousShortNameWithDigest},
		},
		{
			// Fail to lookup image named "sha256" with tag that doesn't exist
			lookup: "sha256:abcdef",
			err:    dockerimages.ErrImageDoesNotExist{Ref: nameTag("sha256", "abcdef")},
		},
		{
			// Lookup with shortened image id
			lookup: ambiguousShortName.Target.Digest.Encoded()[:8],
			img:    nil,
			all:    []images.Image{ambiguousShortName, ambiguousShortNameWithDigest},
		},
		{
			// Lookup an actual image named "sha256" in the default namespace
			lookup: "sha256:defcab",
			img:    &shortNameIsHashAlgorithm,
			all:    []images.Image{shortNameIsHashAlgorithm},
		},
	} {
		tc := tc
		t.Run(tc.lookup, func(t *testing.T) {
			t.Parallel()
			img, all, err := service.resolveAllReferences(ctx, tc.lookup)
			if tc.err == nil {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tc.err.Error())
			}
			if tc.img == nil {
				assert.Assert(t, is.Nil(img))
			} else {
				assert.Assert(t, img != nil)
				assert.Check(t, is.Equal(img.Name, tc.img.Name))
				assert.Check(t, is.Equal(img.Target.Digest, tc.img.Target.Digest))
			}

			assert.Assert(t, is.Len(tc.all, len(all)))

			// Order should match
			for i := range all {
				assert.Check(t, is.Equal(all[i].Name, tc.all[i].Name), "image[%d]", i)
				assert.Check(t, is.Equal(all[i].Target.Digest, tc.all[i].Target.Digest), "image[%d]", i)
			}
		})
	}

	t.Run("fail-inconsistency", func(t *testing.T) {
		service := &ImageService{
			images: &mutateOnGetImageStore{
				Store: service.images,
				getMutations: []images.Image{
					{
						Name:   "docker.io/test/volatile:inconsistent",
						Target: desc(18),
					},
					{
						Name:   "docker.io/test/volatile:inconsistent",
						Target: desc(19),
					},
					{
						Name:   "docker.io/test/volatile:inconsistent",
						Target: desc(20),
					},
					{
						Name:   "docker.io/test/volatile:inconsistent",
						Target: desc(21),
					},
					{
						Name:   "docker.io/test/volatile:inconsistent",
						Target: desc(22),
					},
				},
				t: t,
			},
		}

		_, _, err := service.resolveAllReferences(ctx, "test/volatile:inconsistent")
		assert.ErrorIs(t, err, errInconsistentData)
	})

	t.Run("retry-inconsistency", func(t *testing.T) {
		service := &ImageService{
			images: &mutateOnGetImageStore{
				Store: service.images,
				getMutations: []images.Image{
					{
						Name:   "docker.io/test/volatile:retried",
						Target: desc(16),
					},
					{
						Name:   "docker.io/test/volatile:retried",
						Target: desc(17),
					},
				},
				t: t,
			},
		}

		img, all, err := service.resolveAllReferences(ctx, "test/volatile:retried")
		assert.NilError(t, err)

		assert.Assert(t, img != nil)
		assert.Check(t, is.Equal(img.Name, "docker.io/test/volatile:retried"))
		assert.Check(t, is.Equal(img.Target.Digest, digestFor(17)))
		assert.Assert(t, is.Len(all, 1))
		assert.Check(t, is.Equal(all[0].Name, "docker.io/test/volatile:retried"))
		assert.Check(t, is.Equal(all[0].Target.Digest, digestFor(17)))
	})
}

type mutateOnGetImageStore struct {
	images.Store
	getMutations []images.Image
	t            *testing.T
}

func (m *mutateOnGetImageStore) Get(ctx context.Context, name string) (images.Image, error) {
	img, err := m.Store.Get(ctx, name)
	if len(m.getMutations) > 0 {
		m.Store.Update(ctx, m.getMutations[0])
		m.getMutations = m.getMutations[1:]
		m.t.Logf("Get %s", name)
	}
	return img, err
}

func nameDigest(name string, dgst digest.Digest) reference.Reference {
	named, _ := reference.WithName(name)
	digested, _ := reference.WithDigest(named, dgst)
	return digested
}

func nameTag(name, tag string) reference.Reference {
	named, _ := reference.WithName(name)
	tagged, _ := reference.WithTag(named, tag)
	return tagged
}

func desc(size int64) ocispec.Descriptor {
	return ocispec.Descriptor{
		Digest:    digestFor(size),
		Size:      size,
		MediaType: ocispec.MediaTypeImageIndex,
	}
}

func digestFor(i int64) digest.Digest {
	r := rand.New(rand.NewSource(i))
	dgstr := digest.SHA256.Digester()
	_, err := io.Copy(dgstr.Hash(), io.LimitReader(r, i))
	if err != nil {
		panic(err)
	}
	return dgstr.Digest()
}

func newTestDB(ctx context.Context, t testing.TB) *metadata.DB {
	t.Helper()

	p := filepath.Join(t.TempDir(), "metadata")
	bdb, err := bbolt.Open(p, 0o600, &bbolt.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bdb.Close() })

	mdb := metadata.NewDB(bdb, nil, nil)
	if err := mdb.Init(ctx); err != nil {
		t.Fatal(err)
	}

	return mdb
}

type testSnapshotterService struct {
	snapshots.Snapshotter
}

func (s *testSnapshotterService) Stat(ctx context.Context, key string) (snapshots.Info, error) {
	return snapshots.Info{}, nil
}

func (s *testSnapshotterService) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	return snapshots.Usage{}, nil
}
