package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/archive"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestBuildContextCleanup(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, SameHostDaemon)

	name := "testbuildcontextcleanup"
	entries, err := ioutil.ReadDir(filepath.Join(dockerBasePath, "tmp"))
	if err != nil {
		c.Fatalf("failed to list contents of tmp dir: %s", err)
	}
	_, err = buildImage(name,
		`FROM scratch
        ENTRYPOINT ["/bin/echo"]`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	entriesFinal, err := ioutil.ReadDir(filepath.Join(dockerBasePath, "tmp"))
	if err != nil {
		c.Fatalf("failed to list contents of tmp dir: %s", err)
	}
	if err = compareDirectoryEntries(entries, entriesFinal); err != nil {
		c.Fatalf("context should have been deleted, but wasn't")
	}

}

func (s *DockerSuite) TestBuildContextCleanupFailedBuild(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, SameHostDaemon)

	name := "testbuildcontextcleanup"
	entries, err := ioutil.ReadDir(filepath.Join(dockerBasePath, "tmp"))
	if err != nil {
		c.Fatalf("failed to list contents of tmp dir: %s", err)
	}
	_, err = buildImage(name,
		`FROM scratch
	RUN /non/existing/command`,
		true)
	if err == nil {
		c.Fatalf("expected build to fail, but it didn't")
	}
	entriesFinal, err := ioutil.ReadDir(filepath.Join(dockerBasePath, "tmp"))
	if err != nil {
		c.Fatalf("failed to list contents of tmp dir: %s", err)
	}
	if err = compareDirectoryEntries(entries, entriesFinal); err != nil {
		c.Fatalf("context should have been deleted, but wasn't")
	}

}

func (s *DockerSuite) TestBuildContextTarGzip(c *check.C) {
	testContextTar(c, archive.Gzip)
}

func (s *DockerSuite) TestBuildContextTarNoCompression(c *check.C) {
	testContextTar(c, archive.Uncompressed)
}

func (s *DockerSuite) TestBuildNoContext(c *check.C) {
	testRequires(c, DaemonIsLinux)
	buildCmd := exec.Command(dockerBinary, "build", "-t", "nocontext", "-")
	buildCmd.Stdin = strings.NewReader("FROM busybox\nCMD echo ok\n")

	if out, _, err := runCommandWithOutput(buildCmd); err != nil {
		c.Fatalf("build failed to complete: %v %v", out, err)
	}

	if out, _ := dockerCmd(c, "run", "--rm", "nocontext"); out != "ok\n" {
		c.Fatalf("run produced invalid output: %q, expected %q", out, "ok")
	}
}

func testContextTar(c *check.C, compression archive.Compression) {
	testRequires(c, DaemonIsLinux)
	ctx, err := fakeContext(
		`FROM busybox
ADD foo /foo
CMD ["cat", "/foo"]`,
		map[string]string{
			"foo": "bar",
		},
	)
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()
	context, err := archive.Tar(ctx.Dir, compression)
	if err != nil {
		c.Fatalf("failed to build context tar: %v", err)
	}
	name := "contexttar"
	buildCmd := exec.Command(dockerBinary, "build", "-t", name, "-")
	buildCmd.Stdin = context

	if out, _, err := runCommandWithOutput(buildCmd); err != nil {
		c.Fatalf("build failed to complete: %v %v", out, err)
	}
}

func (s *DockerTrustSuite) TestBuildContextDirIsSymlink(c *check.C) {
	testRequires(c, DaemonIsLinux)
	tempDir, err := ioutil.TempDir("", "test-build-dir-is-symlink-")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tempDir)

	// Make a real context directory in this temp directory with a simple
	// Dockerfile.
	realContextDirname := filepath.Join(tempDir, "context")
	if err := os.Mkdir(realContextDirname, os.FileMode(0755)); err != nil {
		c.Fatal(err)
	}

	if err = ioutil.WriteFile(
		filepath.Join(realContextDirname, "Dockerfile"),
		[]byte(`
			FROM busybox
			RUN echo hello world
		`),
		os.FileMode(0644),
	); err != nil {
		c.Fatal(err)
	}

	// Make a symlink to the real context directory.
	contextSymlinkName := filepath.Join(tempDir, "context_link")
	if err := os.Symlink(realContextDirname, contextSymlinkName); err != nil {
		c.Fatal(err)
	}

	// Executing the build with the symlink as the specified context should
	// *not* fail.
	if out, exitStatus := dockerCmd(c, "build", contextSymlinkName); exitStatus != 0 {
		c.Fatalf("build failed with exit status %d: %s", exitStatus, out)
	}
}

// Issue #5270 - ensure we throw a better error than "unexpected EOF"
// when we can't access files in the context.
func (s *DockerSuite) TestBuildWithInaccessibleFilesInContext(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, UnixCli) // test uses chown/chmod: not available on windows

	{
		name := "testbuildinaccessiblefiles"
		ctx, err := fakeContext("FROM scratch\nADD . /foo/", map[string]string{"fileWithoutReadAccess": "foo"})
		if err != nil {
			c.Fatal(err)
		}
		defer ctx.Close()
		// This is used to ensure we detect inaccessible files early during build in the cli client
		pathToFileWithoutReadAccess := filepath.Join(ctx.Dir, "fileWithoutReadAccess")

		if err = os.Chown(pathToFileWithoutReadAccess, 0, 0); err != nil {
			c.Fatalf("failed to chown file to root: %s", err)
		}
		if err = os.Chmod(pathToFileWithoutReadAccess, 0700); err != nil {
			c.Fatalf("failed to chmod file to 700: %s", err)
		}
		buildCmd := exec.Command("su", "unprivilegeduser", "-c", fmt.Sprintf("%s build -t %s .", dockerBinary, name))
		buildCmd.Dir = ctx.Dir
		out, _, err := runCommandWithOutput(buildCmd)
		if err == nil {
			c.Fatalf("build should have failed: %s %s", err, out)
		}

		// check if we've detected the failure before we started building
		if !strings.Contains(out, "no permission to read from ") {
			c.Fatalf("output should've contained the string: no permission to read from but contained: %s", out)
		}

		if !strings.Contains(out, "Error checking context") {
			c.Fatalf("output should've contained the string: Error checking context")
		}
	}
	{
		name := "testbuildinaccessibledirectory"
		ctx, err := fakeContext("FROM scratch\nADD . /foo/", map[string]string{"directoryWeCantStat/bar": "foo"})
		if err != nil {
			c.Fatal(err)
		}
		defer ctx.Close()
		// This is used to ensure we detect inaccessible directories early during build in the cli client
		pathToDirectoryWithoutReadAccess := filepath.Join(ctx.Dir, "directoryWeCantStat")
		pathToFileInDirectoryWithoutReadAccess := filepath.Join(pathToDirectoryWithoutReadAccess, "bar")

		if err = os.Chown(pathToDirectoryWithoutReadAccess, 0, 0); err != nil {
			c.Fatalf("failed to chown directory to root: %s", err)
		}
		if err = os.Chmod(pathToDirectoryWithoutReadAccess, 0444); err != nil {
			c.Fatalf("failed to chmod directory to 444: %s", err)
		}
		if err = os.Chmod(pathToFileInDirectoryWithoutReadAccess, 0700); err != nil {
			c.Fatalf("failed to chmod file to 700: %s", err)
		}

		buildCmd := exec.Command("su", "unprivilegeduser", "-c", fmt.Sprintf("%s build -t %s .", dockerBinary, name))
		buildCmd.Dir = ctx.Dir
		out, _, err := runCommandWithOutput(buildCmd)
		if err == nil {
			c.Fatalf("build should have failed: %s %s", err, out)
		}

		// check if we've detected the failure before we started building
		if !strings.Contains(out, "can't stat") {
			c.Fatalf("output should've contained the string: can't access %s", out)
		}

		if !strings.Contains(out, "Error checking context") {
			c.Fatalf("output should've contained the string: Error checking context\ngot:%s", out)
		}

	}
	{
		name := "testlinksok"
		ctx, err := fakeContext("FROM scratch\nADD . /foo/", nil)
		if err != nil {
			c.Fatal(err)
		}
		defer ctx.Close()

		target := "../../../../../../../../../../../../../../../../../../../azA"
		if err := os.Symlink(filepath.Join(ctx.Dir, "g"), target); err != nil {
			c.Fatal(err)
		}
		defer os.Remove(target)
		// This is used to ensure we don't follow links when checking if everything in the context is accessible
		// This test doesn't require that we run commands as an unprivileged user
		if _, err := buildImageFromContext(name, ctx, true); err != nil {
			c.Fatal(err)
		}
	}
	{
		name := "testbuildignoredinaccessible"
		ctx, err := fakeContext("FROM scratch\nADD . /foo/",
			map[string]string{
				"directoryWeCantStat/bar": "foo",
				".dockerignore":           "directoryWeCantStat",
			})
		if err != nil {
			c.Fatal(err)
		}
		defer ctx.Close()
		// This is used to ensure we don't try to add inaccessible files when they are ignored by a .dockerignore pattern
		pathToDirectoryWithoutReadAccess := filepath.Join(ctx.Dir, "directoryWeCantStat")
		pathToFileInDirectoryWithoutReadAccess := filepath.Join(pathToDirectoryWithoutReadAccess, "bar")
		if err = os.Chown(pathToDirectoryWithoutReadAccess, 0, 0); err != nil {
			c.Fatalf("failed to chown directory to root: %s", err)
		}
		if err = os.Chmod(pathToDirectoryWithoutReadAccess, 0444); err != nil {
			c.Fatalf("failed to chmod directory to 755: %s", err)
		}
		if err = os.Chmod(pathToFileInDirectoryWithoutReadAccess, 0700); err != nil {
			c.Fatalf("failed to chmod file to 444: %s", err)
		}

		buildCmd := exec.Command("su", "unprivilegeduser", "-c", fmt.Sprintf("%s build -t %s .", dockerBinary, name))
		buildCmd.Dir = ctx.Dir
		if out, _, err := runCommandWithOutput(buildCmd); err != nil {
			c.Fatalf("build should have worked: %s %s", err, out)
		}

	}
}

func (s *DockerSuite) TestBuildDockerfileOutsideContext(c *check.C) {
	testRequires(c, UnixCli) // uses os.Symlink: not implemented in windows at the time of writing (go-1.4.2)
	testRequires(c, DaemonIsLinux)

	name := "testbuilddockerfileoutsidecontext"
	tmpdir, err := ioutil.TempDir("", name)
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpdir)
	ctx := filepath.Join(tmpdir, "context")
	if err := os.MkdirAll(ctx, 0755); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(ctx, "Dockerfile"), []byte("FROM scratch\nENV X Y"), 0644); err != nil {
		c.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		c.Fatal(err)
	}
	defer os.Chdir(wd)
	if err := os.Chdir(ctx); err != nil {
		c.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(tmpdir, "outsideDockerfile"), []byte("FROM scratch\nENV x y"), 0644); err != nil {
		c.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "outsideDockerfile"), filepath.Join(ctx, "dockerfile1")); err != nil {
		c.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(tmpdir, "outsideDockerfile"), filepath.Join(ctx, "dockerfile2")); err != nil {
		c.Fatal(err)
	}

	for _, dockerfilePath := range []string{
		filepath.Join("..", "outsideDockerfile"),
		filepath.Join(ctx, "dockerfile1"),
		filepath.Join(ctx, "dockerfile2"),
	} {
		out, _, err := dockerCmdWithError("build", "-t", name, "--no-cache", "-f", dockerfilePath, ".")
		if err == nil {
			c.Fatalf("Expected error with %s. Out: %s", dockerfilePath, out)
		}
		if !strings.Contains(out, "must be within the build context") && !strings.Contains(out, "Cannot locate Dockerfile") {
			c.Fatalf("Unexpected error with %s. Out: %s", dockerfilePath, out)
		}
		deleteImages(name)
	}

	os.Chdir(tmpdir)

	// Path to Dockerfile should be resolved relative to working directory, not relative to context.
	// There is a Dockerfile in the context, but since there is no Dockerfile in the current directory, the following should fail
	out, _, err := dockerCmdWithError("build", "-t", name, "--no-cache", "-f", "Dockerfile", ctx)
	if err == nil {
		c.Fatalf("Expected error. Out: %s", out)
	}
}

func (s *DockerSuite) TestBuildForbiddenContextPath(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildforbidpath"
	ctx, err := fakeContext(`FROM scratch
        ADD ../../ test/
        `,
		map[string]string{
			"test.txt":  "test1",
			"other.txt": "other",
		})
	if err != nil {
		c.Fatal(err)
	}
	defer ctx.Close()

	expected := "Forbidden path outside the build context: ../../ "
	if _, err := buildImageFromContext(name, ctx, true); err == nil || !strings.Contains(err.Error(), expected) {
		c.Fatalf("Wrong error: (should contain \"%s\") got:\n%v", expected, err)
	}

}

func (s *DockerSuite) TestBuildFromGIT(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildfromgit"
	git, err := newFakeGit("repo", map[string]string{
		"Dockerfile": `FROM busybox
					ADD first /first
					RUN [ -f /first ]
					MAINTAINER docker`,
		"first": "test git data",
	}, true)
	if err != nil {
		c.Fatal(err)
	}
	defer git.Close()

	_, err = buildImageFromPath(name, git.RepoURL, true)
	if err != nil {
		c.Fatal(err)
	}
	res, err := inspectField(name, "Author")
	if err != nil {
		c.Fatal(err)
	}
	if res != "docker" {
		c.Fatalf("Maintainer should be docker, got %s", res)
	}
}

func (s *DockerSuite) TestBuildFromGITWithContext(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildfromgit"
	git, err := newFakeGit("repo", map[string]string{
		"docker/Dockerfile": `FROM busybox
					ADD first /first
					RUN [ -f /first ]
					MAINTAINER docker`,
		"docker/first": "test git data",
	}, true)
	if err != nil {
		c.Fatal(err)
	}
	defer git.Close()

	u := fmt.Sprintf("%s#master:docker", git.RepoURL)
	_, err = buildImageFromPath(name, u, true)
	if err != nil {
		c.Fatal(err)
	}
	res, err := inspectField(name, "Author")
	if err != nil {
		c.Fatal(err)
	}
	if res != "docker" {
		c.Fatalf("Maintainer should be docker, got %s", res)
	}
}

func (s *DockerSuite) TestBuildFromGITwithF(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildfromgitwithf"
	git, err := newFakeGit("repo", map[string]string{
		"myApp/myDockerfile": `FROM busybox
					RUN echo hi from Dockerfile`,
	}, true)
	if err != nil {
		c.Fatal(err)
	}
	defer git.Close()

	out, _, err := dockerCmdWithError("build", "-t", name, "--no-cache", "-f", "myApp/myDockerfile", git.RepoURL)
	if err != nil {
		c.Fatalf("Error on build. Out: %s\nErr: %v", out, err)
	}

	if !strings.Contains(out, "hi from Dockerfile") {
		c.Fatalf("Missing expected output, got:\n%s", out)
	}
}

func (s *DockerSuite) TestBuildFromRemoteTarball(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildfromremotetarball"

	buffer := new(bytes.Buffer)
	tw := tar.NewWriter(buffer)
	defer tw.Close()

	dockerfile := []byte(`FROM busybox
					MAINTAINER docker`)
	if err := tw.WriteHeader(&tar.Header{
		Name: "Dockerfile",
		Size: int64(len(dockerfile)),
	}); err != nil {
		c.Fatalf("failed to write tar file header: %v", err)
	}
	if _, err := tw.Write(dockerfile); err != nil {
		c.Fatalf("failed to write tar file content: %v", err)
	}
	if err := tw.Close(); err != nil {
		c.Fatalf("failed to close tar archive: %v", err)
	}

	server, err := fakeBinaryStorage(map[string]*bytes.Buffer{
		"testT.tar": buffer,
	})
	c.Assert(err, check.IsNil)

	defer server.Close()

	_, err = buildImageFromPath(name, server.URL()+"/testT.tar", true)
	c.Assert(err, check.IsNil)

	res, err := inspectField(name, "Author")
	c.Assert(err, check.IsNil)

	if res != "docker" {
		c.Fatalf("Maintainer should be docker, got %s", res)
	}
}
