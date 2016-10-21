package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/docker/docker/integration-cli/fixtures/load"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func ensureFrozenImagesLinux(t *testing.T) {
	images := []string{"busybox:latest", "hello-world:frozen", "debian:jessie"}
	err := load.FrozenImagesLinux(dockerBinary, images...)
	if err != nil {
		t.Log(dockerCmdWithError("images"))
		t.Fatalf("%+v", err)
	}
	for _, img := range images {
		protectedImages[img] = struct{}{}
	}
}

var ensureSyscallTestOnce sync.Once

func ensureSyscallTest(c *check.C) {
	var doIt bool
	ensureSyscallTestOnce.Do(func() {
		doIt = true
	})
	if !doIt {
		return
	}
	protectedImages["syscall-test:latest"] = struct{}{}

	// if no match, must build in docker, which is significantly slower
	// (slower mostly because of the vfs graphdriver)
	if daemonPlatform != runtime.GOOS {
		ensureSyscallTestBuild(c)
		return
	}

	tmp, err := ioutil.TempDir("", "syscall-test-build")
	c.Assert(err, checker.IsNil, check.Commentf("couldn't create temp dir"))
	defer os.RemoveAll(tmp)

	gcc, err := exec.LookPath("gcc")
	c.Assert(err, checker.IsNil, check.Commentf("could not find gcc"))

	tests := []string{"userns", "ns", "acct", "setuid", "setgid", "socket", "raw"}
	for _, test := range tests {
		out, err := exec.Command(gcc, "-g", "-Wall", "-static", fmt.Sprintf("../contrib/syscall-test/%s.c", test), "-o", fmt.Sprintf("%s/%s-test", tmp, test)).CombinedOutput()
		c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	}

	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		out, err := exec.Command(gcc, "-s", "-m32", "-nostdlib", "../contrib/syscall-test/exit32.s", "-o", tmp+"/"+"exit32-test").CombinedOutput()
		c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	}

	dockerFile := filepath.Join(tmp, "Dockerfile")
	content := []byte(`
	FROM debian:jessie
	COPY . /usr/bin/
	`)
	err = ioutil.WriteFile(dockerFile, content, 600)
	c.Assert(err, checker.IsNil)

	var buildArgs []string
	if arg := os.Getenv("DOCKER_BUILD_ARGS"); strings.TrimSpace(arg) != "" {
		buildArgs = strings.Split(arg, " ")
	}
	buildArgs = append(buildArgs, []string{"-q", "-t", "syscall-test", tmp}...)
	buildArgs = append([]string{"build"}, buildArgs...)
	dockerCmd(c, buildArgs...)
}

func ensureSyscallTestBuild(c *check.C) {
	err := load.FrozenImagesLinux(dockerBinary, "buildpack-deps:jessie")
	c.Assert(err, checker.IsNil)

	var buildArgs []string
	if arg := os.Getenv("DOCKER_BUILD_ARGS"); strings.TrimSpace(arg) != "" {
		buildArgs = strings.Split(arg, " ")
	}
	buildArgs = append(buildArgs, []string{"-q", "-t", "syscall-test", "../contrib/syscall-test"}...)
	buildArgs = append([]string{"build"}, buildArgs...)
	dockerCmd(c, buildArgs...)
}

func ensureNNPTest(c *check.C) {
	protectedImages["nnp-test:latest"] = struct{}{}
	if daemonPlatform != runtime.GOOS {
		ensureNNPTestBuild(c)
		return
	}

	tmp, err := ioutil.TempDir("", "docker-nnp-test")
	c.Assert(err, checker.IsNil)

	gcc, err := exec.LookPath("gcc")
	c.Assert(err, checker.IsNil, check.Commentf("could not find gcc"))

	out, err := exec.Command(gcc, "-g", "-Wall", "-static", "../contrib/nnp-test/nnp-test.c", "-o", filepath.Join(tmp, "nnp-test")).CombinedOutput()
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))

	dockerfile := filepath.Join(tmp, "Dockerfile")
	content := `
	FROM debian:jessie
	COPY . /usr/bin
	RUN chmod +s /usr/bin/nnp-test
	`
	err = ioutil.WriteFile(dockerfile, []byte(content), 600)
	c.Assert(err, checker.IsNil, check.Commentf("could not write Dockerfile for nnp-test image"))

	var buildArgs []string
	if arg := os.Getenv("DOCKER_BUILD_ARGS"); strings.TrimSpace(arg) != "" {
		buildArgs = strings.Split(arg, " ")
	}
	buildArgs = append(buildArgs, []string{"-q", "-t", "nnp-test", tmp}...)
	buildArgs = append([]string{"build"}, buildArgs...)
	dockerCmd(c, buildArgs...)
}

func ensureNNPTestBuild(c *check.C) {
	err := load.FrozenImagesLinux(dockerBinary, "buildpack-deps:jessie")
	c.Assert(err, checker.IsNil)

	var buildArgs []string
	if arg := os.Getenv("DOCKER_BUILD_ARGS"); strings.TrimSpace(arg) != "" {
		buildArgs = strings.Split(arg, " ")
	}
	buildArgs = append(buildArgs, []string{"-q", "-t", "npp-test", "../contrib/nnp-test"}...)
	buildArgs = append([]string{"build"}, buildArgs...)
	dockerCmd(c, buildArgs...)
}
