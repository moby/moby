package image

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/testutil/daemon"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestContentStoreReadSimple(t *testing.T) {
	skip.If(t, !testEnv.UsingSnapshotter())
	ctx := setupTest(t)

	client := testEnv.APIClient()

	inspect, _, err := client.ImageInspectWithRaw(ctx, "busybox")
	assert.NilError(t, err)

	session, err := client.ContentStore(ctx)
	assert.NilError(t, err)

	defer session.Close()

	// This implements the regular containerd interfaces
	var provider content.InfoReaderProvider = session

	assert.Assert(t, inspect.Descriptor != nil)

	b, err := content.ReadBlob(ctx, provider, *inspect.Descriptor)
	assert.NilError(t, err)

	var index ocispec.Index
	assert.NilError(t, json.Unmarshal(b, &index))

	assert.Check(t, is.Equal(index.MediaType, "application/vnd.docker.distribution.manifest.v2+json"))

	t.Run("not existing", func(t *testing.T) {
		desc := ocispec.Descriptor{
			Digest: digest.FromString("this shouldnt exist at all"),
			Size:   100,
		}

		rd, err := provider.ReaderAt(ctx, desc)
		if err == nil {
			rd.Close()
		}
		assert.ErrorType(t, err, errdefs.IsNotFound)
	})
}

func TestContentStoreWrite(t *testing.T) {
	skip.If(t, !testEnv.UsingSnapshotter())
	skip.If(t, testEnv.IsRemoteDaemon())

	ctx := setupTest(t)

	d := daemon.New(t)
	d.Start(t)
	defer d.Stop(t)

	session, err := d.NewClientT(t).ContentStore(ctx)
	assert.NilError(t, err)
	defer session.Close()

	var ingester content.Ingester = session

	wr, err := ingester.Writer(ctx, content.WithRef("test"))
	assert.NilError(t, err)

	_, err = io.Copy(wr, strings.NewReader("hello"))
	assert.NilError(t, err)

	err = wr.Commit(ctx, 5, "")
	assert.NilError(t, err)

	desc := ocispec.Descriptor{
		Digest: wr.Digest(),
		Size:   5,
	}

	t.Run("read back", func(t *testing.T) {
		var provider content.Provider = session

		b, err := content.ReadBlob(ctx, provider, desc)
		assert.NilError(t, err)

		assert.Check(t, is.Equal(string(b), "hello"))
	})

	t.Run("read back after close", func(t *testing.T) {
		// The written content should be gone after the session is closed
		// because it wasn't referenced by anything that would keep it from
		// being GC'd.
		session.Close()
		d.Restart(t)

		newSession, err := d.NewClientT(t).ContentStore(ctx)
		assert.NilError(t, err)

		defer newSession.Close()

		_, err = content.ReadBlob(ctx, newSession, desc)
		assert.ErrorContains(t, err, "not found")
	})

}
