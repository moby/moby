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
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/docker/testutil/fakecontext"
	"github.com/docker/docker/testutil/fixtures/load"
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
	dUserRemap.Start(t, "--userns-remap", "default")
	ctx := context.Background()
	clientUserRemap := dUserRemap.NewClientT(t)
	defer clientUserRemap.Close()

	err = load.FrozenImagesLinux(clientUserRemap, "debian:bullseye-slim")
	assert.NilError(t, err)

	dUserRemapRunning := true
	defer func() {
		if dUserRemapRunning {
			dUserRemap.Stop(t)
			dUserRemap.Cleanup(t)
		}
	}()

	dockerfile := `
		FROM debian:bullseye-slim
		RUN apt-get update && apt-get install -y libcap2-bin --no-install-recommends
		RUN setcap CAP_NET_BIND_SERVICE=+eip /bin/sleep
	`

	source := fakecontext.New(t, "", fakecontext.WithDockerfile(dockerfile))
	defer source.Close()

	resp, err := clientUserRemap.ImageBuild(ctx,
		source.AsTarReader(t),
		types.ImageBuildOptions{
			Tags: []string{imageTag},
		})
	assert.NilError(t, err)
	defer resp.Body.Close()

	buf := bytes.NewBuffer(nil)
	err = jsonmessage.DisplayJSONMessagesStream(resp.Body, buf, 0, false, nil)
	assert.NilError(t, err)

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
	dNoUserRemap.Start(t)
	defer func() {
		dNoUserRemap.Stop(t)
		dNoUserRemap.Cleanup(t)
	}()

	clientNoUserRemap := dNoUserRemap.NewClientT(t)
	defer clientNoUserRemap.Close()

	tarFile, err := os.Open(tmp + "/image.tar")
	assert.NilError(t, err, "failed to open image tar file")
	defer tarFile.Close()

	tarReader := bufio.NewReader(tarFile)
	loadResp, err := clientNoUserRemap.ImageLoad(ctx, tarReader, false)
	assert.NilError(t, err, "failed to load image tar file")
	defer loadResp.Body.Close()
	buf = bytes.NewBuffer(nil)
	err = jsonmessage.DisplayJSONMessagesStream(loadResp.Body, buf, 0, false, nil)
	assert.NilError(t, err)

	cid := container.Run(ctx, t, clientNoUserRemap,
		container.WithImage(imageTag),
		container.WithCmd("/sbin/getcap", "-n", "/bin/sleep"),
	)
	logReader, err := clientNoUserRemap.ContainerLogs(ctx, cid, types.ContainerLogsOptions{
		ShowStdout: true,
	})
	assert.NilError(t, err)
	defer logReader.Close()

	actualStdout := new(bytes.Buffer)
	actualStderr := ioutil.Discard
	_, err = stdcopy.StdCopy(actualStdout, actualStderr, logReader)
	assert.NilError(t, err)
	if strings.TrimSpace(actualStdout.String()) != "/bin/sleep cap_net_bind_service=eip" {
		t.Fatalf("run produced invalid output: %q, expected %q", actualStdout.String(), "/bin/sleep cap_net_bind_service=eip")
	}
}
