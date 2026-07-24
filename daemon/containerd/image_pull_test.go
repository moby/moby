package containerd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/containerd/containerd/v2/core/content"
	c8dimages "github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/platforms"
	"github.com/moby/moby/v2/internal/testutil/specialimage"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
)

func TestEnsurePulledContentAvailableFetchesMissingPlatformLayer(t *testing.T) {
	ctx := namespaces.WithNamespace(t.Context(), "testing-"+t.Name())
	cs := newWritableContentStore(ctx, t)
	imgSvc := fakeImageService(t, ctx, cs)

	srcDir := t.TempDir()
	linuxAmd64 := platforms.MustParse("linux/amd64")
	idx, manifests, err := specialimage.MultiPlatform(srcDir, "multiplatform:latest", []ocispec.Platform{linuxAmd64})
	assert.NilError(t, err)
	assert.Assert(t, len(manifests) == 1)

	img := imagesFromIndex(idx)[0]

	manifest := readManifestBlob(t, srcDir, manifests[0])
	writeBlobFromSpecialImage(ctx, t, cs, srcDir, img.Target)
	writeBlobFromSpecialImage(ctx, t, cs, srcDir, manifests[0])
	writeBlobFromSpecialImage(ctx, t, cs, srcDir, manifest.Config)

	available, _, _, missing, err := c8dimages.Check(ctx, cs, manifests[0], platforms.All)
	assert.NilError(t, err)
	assert.Check(t, available)
	assert.Check(t, len(missing) == 1)

	resolver := staticResolver{
		fetcher: remotes.FetcherFunc(func(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
			return os.Open(blobPathForDescriptor(srcDir, desc))
		}),
	}

	err = imgSvc.ensurePulledContentAvailable(ctx, "multiplatform:latest", resolver, img, platforms.Only(linuxAmd64))
	assert.NilError(t, err)

	available, _, _, missing, err = c8dimages.Check(ctx, cs, manifests[0], platforms.All)
	assert.NilError(t, err)
	assert.Check(t, available)
	assert.Check(t, len(missing) == 0)
}

type staticResolver struct {
	fetcher remotes.Fetcher
}

func (r staticResolver) Resolve(ctx context.Context, ref string) (string, ocispec.Descriptor, error) {
	return ref, ocispec.Descriptor{}, nil
}

func (r staticResolver) Fetcher(ctx context.Context, ref string) (remotes.Fetcher, error) {
	return r.fetcher, nil
}

func (r staticResolver) Pusher(ctx context.Context, ref string) (remotes.Pusher, error) {
	return nil, nil
}

func readManifestBlob(t *testing.T, dir string, desc ocispec.Descriptor) ocispec.Manifest {
	t.Helper()

	mfst, err := readManifest(context.Background(), &blobsDirContentStore{blobs: filepath.Join(dir, "blobs/sha256")}, desc)
	assert.NilError(t, err)
	return mfst
}

func writeBlobFromSpecialImage(ctx context.Context, t *testing.T, cs content.Store, dir string, desc ocispec.Descriptor) {
	t.Helper()

	f, err := os.Open(blobPathForDescriptor(dir, desc))
	assert.NilError(t, err)
	defer f.Close()

	err = content.WriteBlob(ctx, cs, desc.Digest.String(), f, desc)
	assert.NilError(t, err)
}

func blobPathForDescriptor(dir string, desc ocispec.Descriptor) string {
	return filepath.Join(dir, "blobs/sha256", desc.Digest.Encoded())
}
