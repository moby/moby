package build // import "github.com/docker/docker/integration/build"

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/docker/testutil/fakecontext"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

// Implements a test for https://github.com/moby/moby/issues/41723
// Images built in a user-namespaced daemon should have capabilities serialised in
// VFS_CAP_REVISION_2 (no user-namespace root uid) format rather than V3 (that includes
// the root uid).
func TestBuildUserNamespaceValidateCapabilitiesAreV2(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon())
	skip.If(t, !testEnv.IsUserNamespaceInKernel())
	skip.If(t, testEnv.IsRootless())

	const imageTag = "capabilities:1.0"

	tmp, err := ioutil.TempDir("", "integration-")
	assert.NilError(t, err)
	defer os.RemoveAll(tmp)

	dUserRemap := daemon.New(t)
	dUserRemap.StartWithBusybox(t, "--userns-remap", "default")
	dUserRemapRunning := true
	defer func() {
		if dUserRemapRunning {
			dUserRemap.Stop(t)
		}
	}()

	dockerfile := `
		FROM debian:bullseye
		RUN setcap CAP_NET_BIND_SERVICE=+eip /bin/sleep
	`

	ctx := context.Background()
	source := fakecontext.New(t, "", fakecontext.WithDockerfile(dockerfile))
	defer source.Close()

	clientUserRemap := dUserRemap.NewClientT(t)
	resp, err := clientUserRemap.ImageBuild(ctx,
		source.AsTarReader(t),
		types.ImageBuildOptions{
			Tags: []string{imageTag},
		})
	assert.NilError(t, err)
	defer resp.Body.Close()
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		if err != nil && err != io.EOF {
			t.Fatalf("Error reading ImageBuild response: %v", err)
			break
		}
		if n == 0 {
			break
		}
	}

	reader, err := clientUserRemap.ImageSave(ctx, []string{imageTag})
	assert.NilError(t, err, "failed to download capabilities image")
	defer reader.Close()

	tar, err := os.Create(tmp + "/image.tar")
	assert.NilError(t, err, "failed to create image tar file")
	defer tar.Close()

	_, err = io.Copy(tar, reader)
	assert.NilError(t, err, "failed to write image tar file")

	dUserRemap.Stop(t)
	dUserRemap.Cleanup(t)
	dUserRemapRunning = false

	dNoUserRemap := daemon.New(t)
	dNoUserRemap.StartWithBusybox(t)
	defer dNoUserRemap.Stop(t)

	clientNoUserRemap := dNoUserRemap.NewClientT(t)

	tarFile, err := os.Open(tmp + "/image.tar")
	assert.NilError(t, err, "failed to open image tar file")

	tarReader := bufio.NewReader(tarFile)
	loadResp, err := clientNoUserRemap.ImageLoad(ctx, tarReader, false)
	assert.NilError(t, err, "failed to load image tar file")
	defer loadResp.Body.Close()
	for {
		n, err := loadResp.Body.Read(buf)
		if err != nil && err != io.EOF {
			t.Fatalf("Error reading ImageLoad response: %v", err)
			break
		}
		if n == 0 {
			break
		}
	}

	cid := container.Run(ctx, t, clientNoUserRemap,
		container.WithImage(imageTag),
		container.WithCmd("/sbin/getcap", "-n", "/bin/sleep"),
	)
	logReader, err := clientNoUserRemap.ContainerLogs(ctx, cid, types.ContainerLogsOptions{
		ShowStdout: true,
	})
	assert.NilError(t, err)

	actualStdout := new(bytes.Buffer)
	actualStderr := ioutil.Discard
	_, err = stdcopy.StdCopy(actualStdout, actualStderr, logReader)
	assert.NilError(t, err)
	if strings.TrimSpace(actualStdout.String()) != "/bin/sleep cap_net_bind_service=eip" {
		// Activate when fix is merged: https://github.com/moby/moby/pull/41724
		//t.Fatalf("run produced invalid output: %q, expected %q", actualStdout.String(), "/bin/sleep cap_net_bind_service=eip")
		// t.Logf("run produced invalid output (expected until #41724 merges): %q, expected %q",
		// 	actualStdout.String(),
		// 	"/bin/sleep cap_net_bind_service=eip")
	} else {
		// Shouldn't happen until fix is merged: https://github.com/moby/moby/pull/41724
		t.Fatalf("run produced valid output (unexpected until #41724 merges): %q, expected %q",
			actualStdout.String(),
			"/bin/sleep cap_net_bind_service=eip")
	}
}
