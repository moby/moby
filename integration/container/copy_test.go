package container // import "github.com/docker/docker/integration/container"

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/test/fakecontext"
	"github.com/docker/docker/pkg/jsonmessage"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/skip"
)

func TestCopyFromContainerPathDoesNotExist(t *testing.T) {
	defer setupTest(t)()
	skip.If(t, testEnv.OSType == "windows")

	ctx := context.Background()
	apiclient := testEnv.APIClient()
	cid := container.Create(t, ctx, apiclient)

	_, _, err := apiclient.CopyFromContainer(ctx, cid, "/dne")
	assert.Check(t, client.IsErrNotFound(err))
	expected := fmt.Sprintf("No such container:path: %s:%s", cid, "/dne")
	assert.Check(t, is.ErrorContains(err, expected))
}

func TestCopyFromContainerPathIsNotDir(t *testing.T) {
	defer setupTest(t)()
	skip.If(t, testEnv.OSType == "windows")

	ctx := context.Background()
	apiclient := testEnv.APIClient()
	cid := container.Create(t, ctx, apiclient)

	_, _, err := apiclient.CopyFromContainer(ctx, cid, "/etc/passwd/")
	assert.Assert(t, is.ErrorContains(err, "not a directory"))
}

func TestCopyToContainerPathDoesNotExist(t *testing.T) {
	defer setupTest(t)()
	skip.If(t, testEnv.OSType == "windows")

	ctx := context.Background()
	apiclient := testEnv.APIClient()
	cid := container.Create(t, ctx, apiclient)

	err := apiclient.CopyToContainer(ctx, cid, "/dne", nil, types.CopyToContainerOptions{})
	assert.Check(t, client.IsErrNotFound(err))
	expected := fmt.Sprintf("No such container:path: %s:%s", cid, "/dne")
	assert.Check(t, is.ErrorContains(err, expected))
}

func TestCopyToContainerPathIsNotDir(t *testing.T) {
	defer setupTest(t)()
	skip.If(t, testEnv.OSType == "windows")

	ctx := context.Background()
	apiclient := testEnv.APIClient()
	cid := container.Create(t, ctx, apiclient)

	err := apiclient.CopyToContainer(ctx, cid, "/etc/passwd/", nil, types.CopyToContainerOptions{})
	assert.Assert(t, is.ErrorContains(err, "not a directory"))
}

func TestCopyFromContainerRoot(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	defer setupTest(t)()

	ctx := context.Background()
	apiClient := testEnv.APIClient()

	dir, err := ioutil.TempDir("", t.Name())
	assert.NilError(t, err)
	defer os.RemoveAll(dir)

	buildCtx := fakecontext.New(t, dir, fakecontext.WithFile("foo", "hello"), fakecontext.WithFile("baz", "world"), fakecontext.WithDockerfile(`
		FROM scratch
		COPY foo /foo
		COPY baz /bar/baz
		CMD /fake
	`))
	defer buildCtx.Close()

	resp, err := apiClient.ImageBuild(ctx, buildCtx.AsTarReader(t), types.ImageBuildOptions{})
	assert.NilError(t, err)
	defer resp.Body.Close()

	var imageID string
	err = jsonmessage.DisplayJSONMessagesStream(resp.Body, ioutil.Discard, 0, false, func(msg jsonmessage.JSONMessage) {
		var r types.BuildResult
		assert.NilError(t, json.Unmarshal(*msg.Aux, &r))
		imageID = r.ID
	})
	assert.NilError(t, err)
	assert.Assert(t, imageID != "")

	cid := container.Create(ctx, t, apiClient, container.WithImage(imageID))

	rdr, _, err := apiClient.CopyFromContainer(ctx, cid, "/")
	assert.NilError(t, err)
	defer rdr.Close()

	tr := tar.NewReader(rdr)
	expect := map[string]string{
		"/foo":     "hello",
		"/bar/baz": "world",
	}
	found := make(map[string]bool, 2)
	var numFound int
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		assert.NilError(t, err)

		expected, exists := expect[h.Name]
		if !exists {
			// this archive will have extra stuff in it since we are copying from root
			// and docker adds a bunch of stuff
			continue
		}

		numFound++
		found[h.Name] = true

		buf, err := ioutil.ReadAll(tr)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(string(buf), expected))

		if numFound == len(expect) {
			break
		}
	}

	assert.Check(t, found["/foo"], "/foo file not found in archive")
	assert.Check(t, found["/bar/baz"], "/bar/baz file not found in archive")
}
