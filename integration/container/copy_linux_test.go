package container // import "github.com/docker/docker/integration/container"

import (
	"archive/tar"
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/pkg/mount"
	"gotest.tools/assert"
)

// Makes sure copy does not leak shared mounts
// Relates to https://github.com/moby/moby/issues/37624
func TestCopyToContainerDoesNotLeakMounts(t *testing.T) {
	defer setupTest(t)()

	ctx := context.Background()
	apiClient := testEnv.APIClient()

	tmp, err := ioutil.TempDir("", t.Name())
	assert.NilError(t, err)
	defer func() {
		assert.Check(t, os.RemoveAll(tmp), "could not create temp dir")
	}()
	assert.NilError(t, mount.MakeRShared(tmp), "error setting test mountpoint to shared")

	mnt := filepath.Join(tmp, "mnt")
	assert.NilError(t, os.Mkdir(mnt, 0755), "failed to make nested temp dir")

	assert.Assert(t, mount.Mount("tmpfs", mnt, "tmpfs", ""), "failed to create tmpfs on host")
	defer func() {
		assert.Check(t, mount.RecursiveUnmount(tmp))
	}()

	cid := container.Run(t, ctx, apiClient,
		container.WithAutoRemove,
		container.WithCmd("top"),
		container.WithBind(tmp, "/test"),
	)

	buf := bytes.NewBuffer(nil)
	tw := tar.NewWriter(buf)
	data := []byte("hello")
	err = tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "foo",
		Size:     int64(len(data)),
	})
	assert.Assert(t, err, "error writing tar header for test file to copy to container")

	_, err = tw.Write(data)
	assert.NilError(t, err, "error writing tar data file for test file to copy to container")
	assert.NilError(t, tw.Close(), "error closing out tar file")

	err = apiClient.CopyToContainer(ctx, cid, "/tmp/", buf, types.CopyToContainerOptions{})
	assert.Assert(t, err, "copying to container failed")

	exec, err := container.Exec(ctx, apiClient, cid, []string{"/bin/sh", "-c", "cat /proc/mounts | grep '/test/mnt'"})
	assert.NilError(t, err, exec.Stderr())

	out := strings.TrimSpace(exec.Stdout())
	split := strings.Split(out, "\n")
	assert.Equal(t, len(split), 1, out)
}
