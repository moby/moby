package image

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path"
	"testing"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/testutil/registry"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestImagePullPlatformInvalid(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.40"), "experimental in older versions")
	defer setupTest(t)()
	client := testEnv.APIClient()
	ctx := context.Background()

	_, err := client.ImagePull(ctx, "docker.io/library/hello-world:latest", types.ImagePullOptions{Platform: "foobar"})
	assert.Assert(t, err != nil)
	assert.ErrorContains(t, err, "unknown operating system or architecture")
	assert.Assert(t, errdefs.IsInvalidParameter(err))
}

func createTestImage(ctx context.Context, t testing.TB, store content.Store) ocispec.Descriptor {
	w, err := store.Writer(ctx, content.WithRef("layer"))
	assert.NilError(t, err)
	defer w.Close()

	// Empty layer with just a root dir
	const layer = `./0000775000000000000000000000000014201045023007702 5ustar  rootroot`

	_, err = w.Write([]byte(layer))
	assert.NilError(t, err)

	err = w.Commit(ctx, int64(len(layer)), digest.FromBytes([]byte(layer)))
	assert.NilError(t, err)

	layerDigest := w.Digest()
	w.Close()

	img := ocispec.Image{
		Platform: platforms.DefaultSpec(),
		RootFS:   ocispec.RootFS{Type: "layers", DiffIDs: []digest.Digest{layerDigest}},
		Config:   ocispec.ImageConfig{WorkingDir: "/"},
	}
	imgJSON, err := json.Marshal(img)
	assert.NilError(t, err)

	w, err = store.Writer(ctx, content.WithRef("config"))
	assert.NilError(t, err)
	defer w.Close()
	_, err = w.Write(imgJSON)
	assert.NilError(t, err)
	assert.NilError(t, w.Commit(ctx, int64(len(imgJSON)), digest.FromBytes(imgJSON)))

	configDigest := w.Digest()
	w.Close()

	info, err := store.Info(ctx, layerDigest)
	assert.NilError(t, err)

	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType: images.MediaTypeDockerSchema2Manifest,
		Config: ocispec.Descriptor{
			MediaType: images.MediaTypeDockerSchema2Config,
			Digest:    configDigest,
			Size:      int64(len(imgJSON)),
		},
		Layers: []ocispec.Descriptor{{
			MediaType: images.MediaTypeDockerSchema2Layer,
			Digest:    layerDigest,
			Size:      info.Size,
		}},
	}

	manifestJSON, err := json.Marshal(manifest)
	assert.NilError(t, err)

	w, err = store.Writer(ctx, content.WithRef("manifest"))
	assert.NilError(t, err)
	defer w.Close()
	_, err = w.Write(manifestJSON)
	assert.NilError(t, err)
	assert.NilError(t, w.Commit(ctx, int64(len(manifestJSON)), digest.FromBytes(manifestJSON)))

	manifestDigest := w.Digest()
	w.Close()

	return ocispec.Descriptor{
		MediaType: images.MediaTypeDockerSchema2Manifest,
		Digest:    manifestDigest,
		Size:      int64(len(manifestJSON)),
	}
}

// Make sure that pulling by an already cached digest but for a different ref (that should not have that digest)
// verifies with the remote that the digest exists in that repo.
func TestImagePullStoredfDigestForOtherRepo(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.OSType == "windows", "We don't run a test registry on Windows")
	skip.If(t, testEnv.IsRootless, "Rootless has a different view of localhost (needed for test registry access)")
	defer setupTest(t)()

	reg := registry.NewV2(t, registry.WithStdout(os.Stdout), registry.WithStderr(os.Stderr))
	defer reg.Close()
	reg.WaitReady(t)

	ctx := context.Background()

	// First create an image and upload it to our local registry
	// Then we'll download it so that we can make sure the content is available in dockerd's manifest cache.
	// Then we'll try to pull the same digest but with a different repo name.

	dir := t.TempDir()
	store, err := local.NewStore(dir)
	assert.NilError(t, err)

	desc := createTestImage(ctx, t, store)

	remote := path.Join(registry.DefaultURL, "test:latest")

	c8dClient, err := containerd.New("", containerd.WithServices(containerd.WithContentStore(store)))
	assert.NilError(t, err)

	c8dClient.Push(ctx, remote, desc)
	assert.NilError(t, err)

	client := testEnv.APIClient()
	rdr, err := client.ImagePull(ctx, remote, types.ImagePullOptions{})
	assert.NilError(t, err)
	defer rdr.Close()
	io.Copy(io.Discard, rdr)

	// Now, pull a totally different repo with a the same digest
	rdr, err = client.ImagePull(ctx, path.Join(registry.DefaultURL, "other:image@"+desc.Digest.String()), types.ImagePullOptions{})
	if rdr != nil {
		rdr.Close()
	}
	assert.Assert(t, err != nil, "Expected error, got none: %v", err)
	assert.Assert(t, errdefs.IsNotFound(err), err)
}
