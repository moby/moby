package container // import "github.com/docker/docker/integration/container"

import (
	"archive/tar"
	"bytes"
	"os/exec"
	"testing"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/build"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/docker/testutil/fakecontext"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// TestCopyToContainerDecompressOnHost verifies that archive decompression
// happens on the host, not inside the container filesystem.
//
// A malicious container could place a trojan binary at /usr/bin/xz. If
// decompression ran inside RunInFS, dockerd would execute that binary as
// host root. By decompressing before entering the container FS, we ensure
// only host binaries are used.
func TestCopyToContainerDecompressOnHost(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRootless)

	_, err := exec.LookPath("xz")
	skip.If(t, err != nil, "xz not found in PATH")

	t.Parallel()

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "--iptables=false")
	defer d.Stop(t)

	apiClient := d.NewClientT(t)

	buildCtx := fakecontext.New(t, "", fakecontext.WithDockerfile(`
		FROM busybox
		RUN printf '#!/bin/sh\ntouch /compromised\n' > /bin/xz && chmod +x /bin/xz
	`))
	defer buildCtx.Close()

	imageID := build.Do(ctx, t, apiClient, buildCtx)

	cID := container.Run(ctx, t, apiClient, container.WithImage(imageID), container.WithCmd("sleep", "infinity"))
	defer apiClient.ContainerRemove(ctx, cID, containertypes.RemoveOptions{Force: true})

	// Create an xz-compressed tar archive containing a single file.
	var plainTar bytes.Buffer
	tw := tar.NewWriter(&plainTar)
	content := []byte("hello world")
	_ = tw.WriteHeader(&tar.Header{
		Name: "hello.txt",
		Mode: 0o644,
		Size: int64(len(content)),
	})
	_, _ = tw.Write(content)
	_ = tw.Close()

	var xzBuf bytes.Buffer
	xzCmd := exec.Command("xz", "-z")
	xzCmd.Stdin = &plainTar
	xzCmd.Stdout = &xzBuf
	err = xzCmd.Run()
	assert.NilError(t, err, "failed to compress tar with xz")

	err = apiClient.CopyToContainer(ctx, cID, "/tmp", &xzBuf, types.CopyToContainerOptions{})
	assert.NilError(t, err)

	// Verify the file was extracted correctly.
	execRes, err := container.Exec(ctx, apiClient, cID, []string{"cat", "/tmp/hello.txt"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(execRes.ExitCode, 0))
	assert.Check(t, is.Equal(execRes.Stdout(), "hello world"))

	// The malicious /usr/bin/xz inside the container must NOT have been executed.
	execRes, err = container.Exec(ctx, apiClient, cID, []string{"test", "-f", "/compromised"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(execRes.ExitCode, 1), "malicious xz binary inside container was executed")
}
