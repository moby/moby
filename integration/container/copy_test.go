package container // import "github.com/docker/docker/integration/container"

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/build"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/testutil/fakecontext"
	"github.com/moby/go-archive"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestCopyFromContainerPathDoesNotExist(t *testing.T) {
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()
	cid := container.Create(ctx, t, apiClient)

	_, _, err := apiClient.CopyFromContainer(ctx, cid, "/dne")
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
	assert.Check(t, is.ErrorContains(err, "Could not find the file /dne in container "+cid))
}

func TestCopyFromContainerPathIsNotDir(t *testing.T) {
	skip.If(t, testEnv.UsingSnapshotter(), "FIXME: https://github.com/moby/moby/issues/47107")
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()
	cid := container.Create(ctx, t, apiClient)

	path := "/etc/passwd/"
	expected := "not a directory"
	if testEnv.DaemonInfo.OSType == "windows" {
		path = "c:/windows/system32/drivers/etc/hosts/"
		expected = "The filename, directory name, or volume label syntax is incorrect."
	}
	_, _, err := apiClient.CopyFromContainer(ctx, cid, path)
	assert.ErrorContains(t, err, expected)
}

func TestCopyToContainerPathDoesNotExist(t *testing.T) {
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()
	cid := container.Create(ctx, t, apiClient)

	err := apiClient.CopyToContainer(ctx, cid, "/dne", nil, containertypes.CopyToContainerOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
	assert.Check(t, is.ErrorContains(err, "Could not find the file /dne in container "+cid))
}

func TestCopyEmptyFile(t *testing.T) {
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()
	cid := container.Create(ctx, t, apiClient)

	// empty content
	dstDir, _ := makeEmptyArchive(t)
	err := apiClient.CopyToContainer(ctx, cid, dstDir, bytes.NewReader([]byte("")), containertypes.CopyToContainerOptions{})
	assert.NilError(t, err)

	// tar with empty file
	dstDir, preparedArchive := makeEmptyArchive(t)
	err = apiClient.CopyToContainer(ctx, cid, dstDir, preparedArchive, containertypes.CopyToContainerOptions{})
	assert.NilError(t, err)

	// tar with empty file archive mode
	dstDir, preparedArchive = makeEmptyArchive(t)
	err = apiClient.CopyToContainer(ctx, cid, dstDir, preparedArchive, containertypes.CopyToContainerOptions{
		CopyUIDGID: true,
	})
	assert.NilError(t, err)

	// copy from empty file
	rdr, _, err := apiClient.CopyFromContainer(ctx, cid, dstDir)
	assert.NilError(t, err)
	defer rdr.Close()
}

func TestCopyToContainerCopyUIDGID(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()
	imageID := makeTestImage(ctx, t)

	tests := []struct {
		doc      string
		user     string
		expected string
	}{
		{
			doc:      "image default",
			expected: "2375:2376",
		},
		{
			// Align with behavior of docker run, which treats a UID with
			// empty groupname as default (0 (root)).
			//
			//	docker run --rm --user "7777:" alpine id
			//	uid=7777 gid=0(root) groups=0(root)
			doc:      "trailing colon",
			user:     "7777:",
			expected: "7777:0",
		},
		{
			// Align with behavior of docker run, which treats a GID with
			// empty username as default (0 (root)).
			//
			//	docker run --rm --user ":7777" alpine id
			//	uid=0(root) gid=7777 groups=7777
			doc:      "leading colon",
			user:     ":7777",
			expected: "0:7777",
		},
		{
			doc:      "known UID",
			user:     "2375",
			expected: "2375:2376",
		},
		{
			doc:      "unknown UID",
			user:     "7777",
			expected: "7777:0",
		},
		{
			doc:      "UID and GID",
			user:     "2375:2376",
			expected: "2375:2376",
		},
		{
			doc:      "username and groupname",
			user:     "testuser:testgroup",
			expected: "2375:2376",
		},
		{
			doc:      "username",
			user:     "testuser",
			expected: "2375:2376",
		},
		{
			doc:      "username and GID",
			user:     "testuser:7777",
			expected: "2375:7777",
		},
		{
			doc:      "UID and groupname",
			user:     "7777:testgroup",
			expected: "7777:2376",
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			cID := container.Run(ctx, t, apiClient, container.WithImage(imageID), container.WithUser(tc.user))
			defer container.Remove(ctx, t, apiClient, cID, containertypes.RemoveOptions{Force: true})

			// tar with empty file
			dstDir, preparedArchive := makeEmptyArchive(t)
			err := apiClient.CopyToContainer(ctx, cID, dstDir, preparedArchive, containertypes.CopyToContainerOptions{
				CopyUIDGID: true,
			})
			assert.NilError(t, err)

			res, err := container.Exec(ctx, apiClient, cID, []string{"stat", "-c", "%u:%g", "/empty-file.txt"})
			assert.NilError(t, err)
			assert.Equal(t, res.ExitCode, 0)
			assert.Equal(t, strings.TrimSpace(res.Stdout()), tc.expected)
		})
	}
}

func makeTestImage(ctx context.Context, t *testing.T) (imageID string) {
	t.Helper()
	apiClient := testEnv.APIClient()
	tmpDir := t.TempDir()
	buildCtx := fakecontext.New(t, tmpDir, fakecontext.WithDockerfile(`
		FROM busybox
		RUN addgroup -g 2376 testgroup && adduser -D -u 2375 -G testgroup testuser
		USER testuser:testgroup
	`))
	defer buildCtx.Close()

	resp, err := apiClient.ImageBuild(ctx, buildCtx.AsTarReader(t), build.ImageBuildOptions{})
	assert.NilError(t, err)
	defer resp.Body.Close()

	err = jsonmessage.DisplayJSONMessagesStream(resp.Body, io.Discard, 0, false, func(msg jsonmessage.JSONMessage) {
		var r build.Result
		assert.NilError(t, json.Unmarshal(*msg.Aux, &r))
		imageID = r.ID
	})
	assert.NilError(t, err)
	assert.Assert(t, imageID != "")
	return imageID
}

func makeEmptyArchive(t *testing.T) (string, io.ReadCloser) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "empty-file.txt")
	err := os.WriteFile(srcPath, []byte(""), 0o400)
	assert.NilError(t, err)

	// TODO(thaJeztah) Add utilities to the client to make steps below less complicated.
	// Code below is taken from copyToContainer() in docker/cli.
	srcInfo, err := archive.CopyInfoSourcePath(srcPath, false)
	assert.NilError(t, err)

	srcArchive, err := archive.TarResource(srcInfo)
	assert.NilError(t, err)
	t.Cleanup(func() {
		srcArchive.Close()
	})

	ctrPath := "/empty-file.txt"
	dstInfo := archive.CopyInfo{Path: ctrPath}
	dstDir, preparedArchive, err := archive.PrepareArchiveCopy(srcArchive, srcInfo, dstInfo)
	assert.NilError(t, err)
	t.Cleanup(func() {
		preparedArchive.Close()
	})
	return dstDir, preparedArchive
}

func TestCopyToContainerPathIsNotDir(t *testing.T) {
	ctx := setupTest(t)

	apiClient := testEnv.APIClient()
	cid := container.Create(ctx, t, apiClient)

	path := "/etc/passwd/"
	if testEnv.DaemonInfo.OSType == "windows" {
		path = "c:/windows/system32/drivers/etc/hosts/"
	}
	err := apiClient.CopyToContainer(ctx, cid, path, nil, containertypes.CopyToContainerOptions{})
	assert.Check(t, is.ErrorContains(err, "not a directory"))
}

func TestCopyFromContainer(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	ctx := setupTest(t)

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

	resp, err := apiClient.ImageBuild(ctx, buildCtx.AsTarReader(t), build.ImageBuildOptions{})
	assert.NilError(t, err)
	defer resp.Body.Close()

	var imageID string
	err = jsonmessage.DisplayJSONMessagesStream(resp.Body, io.Discard, 0, false, func(msg jsonmessage.JSONMessage) {
		var r build.Result
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
