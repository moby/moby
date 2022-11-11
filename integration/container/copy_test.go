package container // import "github.com/docker/docker/integration/container"

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/testutil/fakecontext"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestCopyFromContainerPathDoesNotExist(t *testing.T) {
	defer setupTest(t)()

	ctx := context.Background()
	apiclient := testEnv.APIClient()
	cid := container.Create(ctx, t, apiclient)

	_, _, err := apiclient.CopyFromContainer(ctx, cid, "/dne")
	assert.Check(t, client.IsErrNotFound(err))
	assert.ErrorContains(t, err, "Could not find the file /dne in container "+cid)
}

func TestCopyFromContainerPathIsNotDir(t *testing.T) {
	defer setupTest(t)()

	ctx := context.Background()
	apiclient := testEnv.APIClient()
	cid := container.Create(ctx, t, apiclient)

	path := "/etc/passwd/"
	expected := "not a directory"
	if testEnv.OSType == "windows" {
		path = "c:/windows/system32/drivers/etc/hosts/"
		expected = "The filename, directory name, or volume label syntax is incorrect."
	}
	_, _, err := apiclient.CopyFromContainer(ctx, cid, path)
	assert.Assert(t, is.ErrorContains(err, expected))
}

func TestCopyToContainerPathDoesNotExist(t *testing.T) {
	defer setupTest(t)()

	ctx := context.Background()
	apiclient := testEnv.APIClient()
	cid := container.Create(ctx, t, apiclient)

	err := apiclient.CopyToContainer(ctx, cid, "/dne", nil, types.CopyToContainerOptions{})
	assert.Check(t, client.IsErrNotFound(err))
	assert.ErrorContains(t, err, "Could not find the file /dne in container "+cid)
}

func TestCopyEmptyFile(t *testing.T) {
	defer setupTest(t)()

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "empty-file.txt")
	err := os.WriteFile(srcPath, []byte(""), 0400)
	assert.NilError(t, err)

	// TODO(thaJeztah) Add utilities to the client to make steps below less complicated.
	// Code below is taken from copyToContainer() in docker/cli.
	srcInfo, err := archive.CopyInfoSourcePath(srcPath, false)
	assert.NilError(t, err)

	srcArchive, err := archive.TarResource(srcInfo)
	assert.NilError(t, err)
	defer srcArchive.Close()

	ctrPath := "/empty-file.txt"
	dstInfo := archive.CopyInfo{Path: ctrPath}
	dstDir, preparedArchive, err := archive.PrepareArchiveCopy(srcArchive, srcInfo, dstInfo)
	assert.NilError(t, err)
	defer preparedArchive.Close()

	ctx := context.Background()
	apiclient := testEnv.APIClient()
	cid := container.Create(ctx, t, apiclient)

	// empty content
	err = apiclient.CopyToContainer(ctx, cid, dstDir, bytes.NewReader([]byte("")), types.CopyToContainerOptions{})
	assert.NilError(t, err)

	// tar with empty file
	err = apiclient.CopyToContainer(ctx, cid, dstDir, preparedArchive, types.CopyToContainerOptions{})
	assert.NilError(t, err)

	// copy from empty file
	rdr, _, err := apiclient.CopyFromContainer(ctx, cid, dstDir)
	assert.NilError(t, err)
	defer rdr.Close()
}

func TestCopyToContainerPathIsNotDir(t *testing.T) {
	defer setupTest(t)()

	ctx := context.Background()
	apiclient := testEnv.APIClient()
	cid := container.Create(ctx, t, apiclient)

	path := "/etc/passwd/"
	if testEnv.OSType == "windows" {
		path = "c:/windows/system32/drivers/etc/hosts/"
	}
	err := apiclient.CopyToContainer(ctx, cid, path, nil, types.CopyToContainerOptions{})
	assert.Assert(t, is.ErrorContains(err, "not a directory"))
}

func TestCopyFromContainer(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	defer setupTest(t)()

	ctx := context.Background()
	apiClient := testEnv.APIClient()

	dir, err := os.MkdirTemp("", t.Name())
	assert.NilError(t, err)
	defer os.RemoveAll(dir)

	buildCtx := fakecontext.New(t, dir, fakecontext.WithFile("foo", "hello"), fakecontext.WithFile("baz", "world"), fakecontext.WithDockerfile(`
		FROM busybox
		COPY foo /foo
		COPY baz /bar/quux/baz
		RUN ln -s notexist /bar/notarget && ln -s quux/baz /bar/filesymlink && ln -s quux /bar/dirsymlink && ln -s / /bar/root
		CMD /fake
	`))
	defer buildCtx.Close()

	resp, err := apiClient.ImageBuild(ctx, buildCtx.AsTarReader(t), types.ImageBuildOptions{})
	assert.NilError(t, err)
	defer resp.Body.Close()

	var imageID string
	err = jsonmessage.DisplayJSONMessagesStream(resp.Body, io.Discard, 0, false, func(msg jsonmessage.JSONMessage) {
		var r types.BuildResult
		assert.NilError(t, json.Unmarshal(*msg.Aux, &r))
		imageID = r.ID
	})
	assert.NilError(t, err)
	assert.Assert(t, imageID != "")

	cid := container.Create(ctx, t, apiClient, container.WithImage(imageID))

	for _, x := range []struct {
		src    string
		expect map[string]string
	}{
		{"/", map[string]string{"/": "", "/foo": "hello", "/bar/quux/baz": "world", "/bar/filesymlink": "", "/bar/dirsymlink": "", "/bar/notarget": ""}},
		{".", map[string]string{"./": "", "./foo": "hello", "./bar/quux/baz": "world", "./bar/filesymlink": "", "./bar/dirsymlink": "", "./bar/notarget": ""}},
		{"/.", map[string]string{"./": "", "./foo": "hello", "./bar/quux/baz": "world", "./bar/filesymlink": "", "./bar/dirsymlink": "", "./bar/notarget": ""}},
		{"./", map[string]string{"./": "", "./foo": "hello", "./bar/quux/baz": "world", "./bar/filesymlink": "", "./bar/dirsymlink": "", "./bar/notarget": ""}},
		{"/./", map[string]string{"./": "", "./foo": "hello", "./bar/quux/baz": "world", "./bar/filesymlink": "", "./bar/dirsymlink": "", "./bar/notarget": ""}},
		{"/bar/root", map[string]string{"root": ""}},
		{"/bar/root/", map[string]string{"root/": "", "root/foo": "hello", "root/bar/quux/baz": "world", "root/bar/filesymlink": "", "root/bar/dirsymlink": "", "root/bar/notarget": ""}},
		{"/bar/root/.", map[string]string{"./": "", "./foo": "hello", "./bar/quux/baz": "world", "./bar/filesymlink": "", "./bar/dirsymlink": "", "./bar/notarget": ""}},

		{"bar/quux", map[string]string{"quux/": "", "quux/baz": "world"}},
		{"bar/quux/", map[string]string{"quux/": "", "quux/baz": "world"}},
		{"bar/quux/.", map[string]string{"./": "", "./baz": "world"}},
		{"bar/quux/baz", map[string]string{"baz": "world"}},

		{"bar/filesymlink", map[string]string{"filesymlink": ""}},
		{"bar/dirsymlink", map[string]string{"dirsymlink": ""}},
		{"bar/dirsymlink/", map[string]string{"dirsymlink/": "", "dirsymlink/baz": "world"}},
		{"bar/dirsymlink/.", map[string]string{"./": "", "./baz": "world"}},
		{"bar/notarget", map[string]string{"notarget": ""}},
	} {
		t.Run(x.src, func(t *testing.T) {
			rdr, _, err := apiClient.CopyFromContainer(ctx, cid, x.src)
			assert.NilError(t, err)
			defer rdr.Close()

			found := make(map[string]bool, len(x.expect))
			var numFound int
			tr := tar.NewReader(rdr)
			for numFound < len(x.expect) {
				h, err := tr.Next()
				if err == io.EOF {
					break
				}
				assert.NilError(t, err)

				expected, exists := x.expect[h.Name]
				if !exists {
					// this archive will have extra stuff in it since we are copying from root
					// and docker adds a bunch of stuff
					continue
				}

				numFound++
				found[h.Name] = true

				buf, err := io.ReadAll(tr)
				if err == nil {
					assert.Check(t, is.Equal(string(buf), expected))
				}
			}

			for f := range x.expect {
				assert.Check(t, found[f], f+" not found in archive")
			}
		})
	}
}
