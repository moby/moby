package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/docker/testutil/fixtures/load"
	"gotest.tools/v3/assert"
)

func ensureSyscallTest(c *testing.T) {
	defer testEnv.ProtectImage(c, "syscall-test:latest")

	// If the image already exists, there's nothing left to do.
	if testEnv.HasExistingImage(c, "syscall-test:latest") {
		return
	}

	// if no match, must build in docker, which is significantly slower
	// (slower mostly because of the vfs graphdriver)
	if testEnv.OSType != runtime.GOOS {
		ensureSyscallTestBuild(c)
		return
	}

	tmp, err := ioutil.TempDir("", "syscall-test-build")
	assert.NilError(c, err, "couldn't create temp dir")
	defer os.RemoveAll(tmp)

	gcc, err := exec.LookPath("gcc")
	assert.NilError(c, err, "could not find gcc")

	tests := []string{"userns", "ns", "acct", "setuid", "setgid", "socket", "raw"}
	for _, test := range tests {
		out, err := exec.Command(gcc, "-g", "-Wall", "-static", fmt.Sprintf("../contrib/syscall-test/%s.c", test), "-o", fmt.Sprintf("%s/%s-test", tmp, test)).CombinedOutput()
		assert.NilError(c, err, string(out))
	}

	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		out, err := exec.Command(gcc, "-s", "-m32", "-nostdlib", "-static", "../contrib/syscall-test/exit32.s", "-o", tmp+"/"+"exit32-test").CombinedOutput()
		assert.NilError(c, err, string(out))
	}

	dockerFile := filepath.Join(tmp, "Dockerfile")
	content := []byte(`
	FROM debian:bullseye
	COPY . /usr/bin/
	`)
	err = ioutil.WriteFile(dockerFile, content, 0600)
	assert.NilError(c, err)

	var buildArgs []string
	if arg := os.Getenv("DOCKER_BUILD_ARGS"); strings.TrimSpace(arg) != "" {
		buildArgs = strings.Split(arg, " ")
	}
	buildArgs = append(buildArgs, []string{"-q", "-t", "syscall-test", tmp}...)
	buildArgs = append([]string{"build"}, buildArgs...)
	dockerCmd(c, buildArgs...)
}

func ensureSyscallTestBuild(c *testing.T) {
	err := load.FrozenImagesLinux(testEnv.APIClient(), "buildpack-deps:buster")
	assert.NilError(c, err)

	var buildArgs []string
	if arg := os.Getenv("DOCKER_BUILD_ARGS"); strings.TrimSpace(arg) != "" {
		buildArgs = strings.Split(arg, " ")
	}
	buildArgs = append(buildArgs, []string{"-q", "-t", "syscall-test", "../contrib/syscall-test"}...)
	buildArgs = append([]string{"build"}, buildArgs...)
	dockerCmd(c, buildArgs...)
}

func ensureNNPTest(c *testing.T) {
	defer testEnv.ProtectImage(c, "nnp-test:latest")

	// If the image already exists, there's nothing left to do.
	if testEnv.HasExistingImage(c, "nnp-test:latest") {
		return
	}

	// if no match, must build in docker, which is significantly slower
	// (slower mostly because of the vfs graphdriver)
	if testEnv.OSType != runtime.GOOS {
		ensureNNPTestBuild(c)
		return
	}

	tmp, err := ioutil.TempDir("", "docker-nnp-test")
	assert.NilError(c, err)

	gcc, err := exec.LookPath("gcc")
	assert.NilError(c, err, "could not find gcc")

	out, err := exec.Command(gcc, "-g", "-Wall", "-static", "../contrib/nnp-test/nnp-test.c", "-o", filepath.Join(tmp, "nnp-test")).CombinedOutput()
	assert.NilError(c, err, string(out))

	dockerfile := filepath.Join(tmp, "Dockerfile")
	content := `
	FROM debian:bullseye
	COPY . /usr/bin
	RUN chmod +s /usr/bin/nnp-test
	`
	err = ioutil.WriteFile(dockerfile, []byte(content), 0600)
	assert.NilError(c, err, "could not write Dockerfile for nnp-test image")

	var buildArgs []string
	if arg := os.Getenv("DOCKER_BUILD_ARGS"); strings.TrimSpace(arg) != "" {
		buildArgs = strings.Split(arg, " ")
	}
	buildArgs = append(buildArgs, []string{"-q", "-t", "nnp-test", tmp}...)
	buildArgs = append([]string{"build"}, buildArgs...)
	dockerCmd(c, buildArgs...)
}

func ensureNNPTestBuild(c *testing.T) {
	err := load.FrozenImagesLinux(testEnv.APIClient(), "buildpack-deps:buster")
	assert.NilError(c, err)

	var buildArgs []string
	if arg := os.Getenv("DOCKER_BUILD_ARGS"); strings.TrimSpace(arg) != "" {
		buildArgs = strings.Split(arg, " ")
	}
	buildArgs = append(buildArgs, []string{"-q", "-t", "npp-test", "../contrib/nnp-test"}...)
	buildArgs = append([]string{"build"}, buildArgs...)
	dockerCmd(c, buildArgs...)
}
