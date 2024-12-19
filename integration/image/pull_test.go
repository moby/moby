package image

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	c8dimages "github.com/containerd/containerd/images"
	"github.com/containerd/platforms"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/docker/testutil/registry"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestImagePullPlatformInvalid(t *testing.T) {
	ctx := setupTest(t)

	client := testEnv.APIClient()

	_, err := client.ImagePull(ctx, "docker.io/library/hello-world:latest", image.PullOptions{Platform: "foobar"})
	assert.Assert(t, err != nil)
	assert.Check(t, is.ErrorContains(err, "unknown operating system or architecture"))
	assert.Check(t, is.ErrorType(err, errdefs.IsInvalidParameter))
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
	assert.Check(t, w.Close())

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
	assert.Check(t, w.Close())

	info, err := store.Info(ctx, layerDigest)
	assert.NilError(t, err)

	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType: c8dimages.MediaTypeDockerSchema2Manifest,
		Config: ocispec.Descriptor{
			MediaType: c8dimages.MediaTypeDockerSchema2Config,
			Digest:    configDigest,
			Size:      int64(len(imgJSON)),
		},
		Layers: []ocispec.Descriptor{{
			MediaType: c8dimages.MediaTypeDockerSchema2Layer,
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
	assert.Check(t, w.Close())

	return ocispec.Descriptor{
		MediaType: c8dimages.MediaTypeDockerSchema2Manifest,
		Digest:    manifestDigest,
		Size:      int64(len(manifestJSON)),
	}
}

// Make sure that pulling by an already cached digest but for a different ref (that should not have that digest)
// verifies with the remote that the digest exists in that repo.
func TestImagePullStoredDigestForOtherRepo(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "We don't run a test registry on Windows")
	skip.If(t, testEnv.IsRootless, "Rootless has a different view of localhost (needed for test registry access)")
	ctx := setupTest(t)

	reg := registry.NewV2(t, registry.WithStdout(os.Stdout), registry.WithStderr(os.Stderr))
	defer reg.Close()
	reg.WaitReady(t)

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
	rdr, err := client.ImagePull(ctx, remote, image.PullOptions{})
	assert.NilError(t, err)
	defer rdr.Close()
	_, err = io.Copy(io.Discard, rdr)
	assert.Check(t, err)

	// Now, pull a totally different repo with a the same digest
	rdr, err = client.ImagePull(ctx, path.Join(registry.DefaultURL, "other:image@"+desc.Digest.String()), image.PullOptions{})
	if rdr != nil {
		assert.Check(t, rdr.Close())
	}
	assert.Assert(t, err != nil, "Expected error, got none: %v", err)
	assert.Assert(t, errdefs.IsNotFound(err), err)
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
}

// TestImagePullNonExisting pulls non-existing images from the central registry, with different
// combinations of implicit tag and library prefix.
func TestImagePullNonExisting(t *testing.T) {
	ctx := setupTest(t)

	for _, ref := range []string{
		"asdfasdf:foobar",
		"library/asdfasdf:foobar",
		"asdfasdf",
		"asdfasdf:latest",
		"library/asdfasdf",
		"library/asdfasdf:latest",
	} {
		all := strings.Contains(ref, ":")
		t.Run(ref, func(t *testing.T) {
			t.Parallel()

			client := testEnv.APIClient()
			rdr, err := client.ImagePull(ctx, ref, image.PullOptions{
				All: all,
			})
			if err == nil {
				rdr.Close()
			}

			expectedMsg := fmt.Sprintf("pull access denied for %s, repository does not exist or may require 'docker login'", "asdfasdf")
			assert.Check(t, is.ErrorContains(err, expectedMsg))
			assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
			if all {
				// pull -a on a nonexistent registry should fall back as well
				assert.Check(t, !strings.Contains(err.Error(), "unauthorized"), `message should not contain "unauthorized"`)
			}
		})
	}
}

func TestImagePullKeepOldAsDangling(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "Can't run new daemons on Windows")

	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Cleanup(t)

	apiClient := d.NewClientT(t)

	inspect1, _, err := apiClient.ImageInspectWithRaw(ctx, "busybox:latest")
	assert.NilError(t, err)

	prevID := inspect1.ID

	t.Log(inspect1)

	assert.NilError(t, apiClient.ImageTag(ctx, "busybox:latest", "alpine:latest"))

	_, err = apiClient.ImageRemove(ctx, "busybox:latest", image.RemoveOptions{})
	assert.NilError(t, err)

	rc, err := apiClient.ImagePull(ctx, "alpine:latest", image.PullOptions{})
	assert.NilError(t, err)

	defer rc.Close()

	var b bytes.Buffer
	_, _ = io.Copy(&b, rc)

	t.Log(b.String())

	_, _, err = apiClient.ImageInspectWithRaw(ctx, prevID)
	assert.NilError(t, err)
}
