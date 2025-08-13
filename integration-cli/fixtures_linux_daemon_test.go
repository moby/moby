package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/moby/moby/v2/integration-cli/cli"
	"github.com/moby/moby/v2/testutil/fixtures/load"
	"gotest.tools/v3/assert"
)

func ensureSyscallTest(ctx context.Context, t *testing.T) {
	defer testEnv.ProtectImage(t, "syscall-test:latest")

	// If the image already exists, there's nothing left to do.
	if testEnv.HasExistingImage(t, "syscall-test:latest") {
		return
	}

	// if no match, must build in docker, which is significantly slower
	// (slower mostly because of the vfs graphdriver)
	if testEnv.DaemonInfo.OSType != runtime.GOOS {
		ensureSyscallTestBuild(ctx, t)
		return
	}

	tmp, err := os.MkdirTemp("", "syscall-test-build")
	assert.NilError(t, err, "couldn't create temp dir")
	defer os.RemoveAll(tmp)

	gcc, err := exec.LookPath("gcc")
	assert.NilError(t, err, "could not find gcc")

	tests := []string{"userns", "ns", "acct", "setuid", "setgid", "socket", "raw"}
	for _, test := range tests {
		out, err := exec.Command(gcc, "-g", "-Wall", "-static", fmt.Sprintf("../contrib/syscall-test/%s.c", test), "-o", fmt.Sprintf("%s/%s-test", tmp, test)).CombinedOutput()
		assert.NilError(t, err, string(out))
	}

	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		out, err := exec.Command(gcc, "-s", "-m32", "-nostdlib", "-static", "../contrib/syscall-test/exit32.s", "-o", tmp+"/"+"exit32-test").CombinedOutput()
		assert.NilError(t, err, string(out))
	}

	dockerFile := filepath.Join(tmp, "Dockerfile")
	content := []byte(`
	FROM debian:trixie-slim
	COPY . /usr/bin/
	`)
	err = os.WriteFile(dockerFile, content, 0o600)
	assert.NilError(t, err)

	var buildArgs []string
	if arg := os.Getenv("DOCKER_BUILD_ARGS"); strings.TrimSpace(arg) != "" {
		buildArgs = strings.Split(arg, " ")
	}
	buildArgs = append(buildArgs, []string{"-q", "-t", "syscall-test", tmp}...)
	buildArgs = append([]string{"build"}, buildArgs...)
	cli.DockerCmd(t, buildArgs...)
}

func ensureSyscallTestBuild(ctx context.Context, t *testing.T) {
	err := load.FrozenImagesLinux(ctx, testEnv.APIClient(), "debian:trixie-slim")
	assert.NilError(t, err)

	var buildArgs []string
	if arg := os.Getenv("DOCKER_BUILD_ARGS"); strings.TrimSpace(arg) != "" {
		buildArgs = strings.Split(arg, " ")
	}
	buildArgs = append(buildArgs, []string{"-q", "-t", "syscall-test", "../contrib/syscall-test"}...)
	buildArgs = append([]string{"build"}, buildArgs...)
	cli.DockerCmd(t, buildArgs...)
}

func ensureNNPTest(ctx context.Context, t *testing.T) {
	defer testEnv.ProtectImage(t, "nnp-test:latest")

	// If the image already exists, there's nothing left to do.
	if testEnv.HasExistingImage(t, "nnp-test:latest") {
		return
	}

	// if no match, must build in docker, which is significantly slower
	// (slower mostly because of the vfs graphdriver)
	if testEnv.DaemonInfo.OSType != runtime.GOOS {
		ensureNNPTestBuild(ctx, t)
		return
	}

	tmp, err := os.MkdirTemp("", "docker-nnp-test")
	assert.NilError(t, err)

	gcc, err := exec.LookPath("gcc")
	assert.NilError(t, err, "could not find gcc")

	out, err := exec.Command(gcc, "-g", "-Wall", "-static", "../contrib/nnp-test/nnp-test.c", "-o", filepath.Join(tmp, "nnp-test")).CombinedOutput()
	assert.NilError(t, err, string(out))

	dockerfile := filepath.Join(tmp, "Dockerfile")
	content := `
	FROM debian:trixie-slim
	COPY . /usr/bin
	RUN chmod +s /usr/bin/nnp-test
	`
	err = os.WriteFile(dockerfile, []byte(content), 0o600)
	assert.NilError(t, err, "could not write Dockerfile for nnp-test image")

	var buildArgs []string
	if arg := os.Getenv("DOCKER_BUILD_ARGS"); strings.TrimSpace(arg) != "" {
		buildArgs = strings.Split(arg, " ")
	}
	buildArgs = append(buildArgs, []string{"-q", "-t", "nnp-test", tmp}...)
	buildArgs = append([]string{"build"}, buildArgs...)
	cli.DockerCmd(t, buildArgs...)
}

func ensureNNPTestBuild(ctx context.Context, t *testing.T) {
	err := load.FrozenImagesLinux(ctx, testEnv.APIClient(), "debian:trixie-slim")
	assert.NilError(t, err)

	var buildArgs []string
	if arg := os.Getenv("DOCKER_BUILD_ARGS"); strings.TrimSpace(arg) != "" {
		buildArgs = strings.Split(arg, " ")
	}
	buildArgs = append(buildArgs, []string{"-q", "-t", "npp-test", "../contrib/nnp-test"}...)
	buildArgs = append([]string{"build"}, buildArgs...)
	cli.DockerCmd(t, buildArgs...)
}
