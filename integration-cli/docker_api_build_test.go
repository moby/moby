package main

import (
	"archive/tar"
	"bytes"
	"net/http"
	"strings"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestBuildApiDockerfilePath(c *check.C) {
	// Test to make sure we stop people from trying to leave the
	// build context when specifying the path to the dockerfile
	buffer := new(bytes.Buffer)
	tw := tar.NewWriter(buffer)
	defer tw.Close()

	dockerfile := []byte("FROM busybox")
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

	res, body, err := sockRequestRaw("POST", "/build?dockerfile=../Dockerfile", buffer, "application/x-tar")
	c.Assert(err, check.IsNil)
	c.Assert(res.StatusCode, check.Equals, http.StatusInternalServerError)

	out, err := readBody(body)
	if err != nil {
		c.Fatal(err)
	}

	if !strings.Contains(string(out), "Forbidden path outside the build context") {
		c.Fatalf("Didn't complain about leaving build context: %s", out)
	}
}

func (s *DockerSuite) TestBuildApiDockerFileRemote(c *check.C) {
	testRequires(c, NotUserNamespace)
	testRequires(c, DaemonIsLinux)
	server, err := fakeStorage(map[string]string{
		"testD": `FROM busybox
COPY * /tmp/
RUN find / -name ba*
RUN find /tmp/`,
	})
	if err != nil {
		c.Fatal(err)
	}
	defer server.Close()

	res, body, err := sockRequestRaw("POST", "/build?dockerfile=baz&remote="+server.URL()+"/testD", nil, "application/json")
	c.Assert(err, check.IsNil)
	c.Assert(res.StatusCode, check.Equals, http.StatusOK)

	buf, err := readBody(body)
	if err != nil {
		c.Fatal(err)
	}

	// Make sure Dockerfile exists.
	// Make sure 'baz' doesn't exist ANYWHERE despite being mentioned in the URL
	out := string(buf)
	if !strings.Contains(out, "/tmp/Dockerfile") ||
		strings.Contains(out, "baz") {
		c.Fatalf("Incorrect output: %s", out)
	}
}

func (s *DockerSuite) TestBuildApiRemoteTarballContext(c *check.C) {
	testRequires(c, DaemonIsLinux)
	buffer := new(bytes.Buffer)
	tw := tar.NewWriter(buffer)
	defer tw.Close()

	dockerfile := []byte("FROM busybox")
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

	res, b, err := sockRequestRaw("POST", "/build?remote="+server.URL()+"/testT.tar", nil, "application/tar")
	c.Assert(err, check.IsNil)
	c.Assert(res.StatusCode, check.Equals, http.StatusOK)
	b.Close()
}

func (s *DockerSuite) TestBuildApiRemoteTarballContextWithCustomDockerfile(c *check.C) {
	testRequires(c, DaemonIsLinux)
	buffer := new(bytes.Buffer)
	tw := tar.NewWriter(buffer)
	defer tw.Close()

	dockerfile := []byte(`FROM busybox
RUN echo 'wrong'`)
	if err := tw.WriteHeader(&tar.Header{
		Name: "Dockerfile",
		Size: int64(len(dockerfile)),
	}); err != nil {
		c.Fatalf("failed to write tar file header: %v", err)
	}
	if _, err := tw.Write(dockerfile); err != nil {
		c.Fatalf("failed to write tar file content: %v", err)
	}

	custom := []byte(`FROM busybox
RUN echo 'right'
`)
	if err := tw.WriteHeader(&tar.Header{
		Name: "custom",
		Size: int64(len(custom)),
	}); err != nil {
		c.Fatalf("failed to write tar file header: %v", err)
	}
	if _, err := tw.Write(custom); err != nil {
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
	url := "/build?dockerfile=custom&remote=" + server.URL() + "/testT.tar"
	res, body, err := sockRequestRaw("POST", url, nil, "application/tar")
	c.Assert(err, check.IsNil)
	c.Assert(res.StatusCode, check.Equals, http.StatusOK)

	defer body.Close()
	content, err := readBody(body)
	c.Assert(err, check.IsNil)

	if strings.Contains(string(content), "wrong") {
		c.Fatalf("Build used the wrong dockerfile.")
	}
}

func (s *DockerSuite) TestBuildApiLowerDockerfile(c *check.C) {
	testRequires(c, DaemonIsLinux)
	git, err := newFakeGit("repo", map[string]string{
		"dockerfile": `FROM busybox
RUN echo from dockerfile`,
	}, false)
	if err != nil {
		c.Fatal(err)
	}
	defer git.Close()

	res, body, err := sockRequestRaw("POST", "/build?remote="+git.RepoURL, nil, "application/json")
	c.Assert(err, check.IsNil)
	c.Assert(res.StatusCode, check.Equals, http.StatusOK)

	buf, err := readBody(body)
	if err != nil {
		c.Fatal(err)
	}

	out := string(buf)
	if !strings.Contains(out, "from dockerfile") {
		c.Fatalf("Incorrect output: %s", out)
	}
}

func (s *DockerSuite) TestBuildApiBuildGitWithF(c *check.C) {
	testRequires(c, DaemonIsLinux)
	git, err := newFakeGit("repo", map[string]string{
		"baz": `FROM busybox
RUN echo from baz`,
		"Dockerfile": `FROM busybox
RUN echo from Dockerfile`,
	}, false)
	if err != nil {
		c.Fatal(err)
	}
	defer git.Close()

	// Make sure it tries to 'dockerfile' query param value
	res, body, err := sockRequestRaw("POST", "/build?dockerfile=baz&remote="+git.RepoURL, nil, "application/json")
	c.Assert(err, check.IsNil)
	c.Assert(res.StatusCode, check.Equals, http.StatusOK)

	buf, err := readBody(body)
	if err != nil {
		c.Fatal(err)
	}

	out := string(buf)
	if !strings.Contains(out, "from baz") {
		c.Fatalf("Incorrect output: %s", out)
	}
}

func (s *DockerSuite) TestBuildApiDoubleDockerfile(c *check.C) {
	testRequires(c, UnixCli) // dockerfile overwrites Dockerfile on Windows
	git, err := newFakeGit("repo", map[string]string{
		"Dockerfile": `FROM busybox
RUN echo from Dockerfile`,
		"dockerfile": `FROM busybox
RUN echo from dockerfile`,
	}, false)
	if err != nil {
		c.Fatal(err)
	}
	defer git.Close()

	// Make sure it tries to 'dockerfile' query param value
	res, body, err := sockRequestRaw("POST", "/build?remote="+git.RepoURL, nil, "application/json")
	c.Assert(err, check.IsNil)
	c.Assert(res.StatusCode, check.Equals, http.StatusOK)

	buf, err := readBody(body)
	if err != nil {
		c.Fatal(err)
	}

	out := string(buf)
	if !strings.Contains(out, "from Dockerfile") {
		c.Fatalf("Incorrect output: %s", out)
	}
}

func (s *DockerSuite) TestBuildApiDockerfileSymlink(c *check.C) {
	// Test to make sure we stop people from trying to leave the
	// build context when specifying a symlink as the path to the dockerfile
	buffer := new(bytes.Buffer)
	tw := tar.NewWriter(buffer)
	defer tw.Close()

	if err := tw.WriteHeader(&tar.Header{
		Name:     "Dockerfile",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
	}); err != nil {
		c.Fatalf("failed to write tar file header: %v", err)
	}
	if err := tw.Close(); err != nil {
		c.Fatalf("failed to close tar archive: %v", err)
	}

	res, body, err := sockRequestRaw("POST", "/build", buffer, "application/x-tar")
	c.Assert(err, check.IsNil)
	c.Assert(res.StatusCode, check.Equals, http.StatusInternalServerError)

	out, err := readBody(body)
	if err != nil {
		c.Fatal(err)
	}

	// The reason the error is "Cannot locate specified Dockerfile" is because
	// in the builder, the symlink is resolved within the context, therefore
	// Dockerfile -> /etc/passwd becomes etc/passwd from the context which is
	// a nonexistent file.
	if !strings.Contains(string(out), "Cannot locate specified Dockerfile: Dockerfile") {
		c.Fatalf("Didn't complain about leaving build context: %s", out)
	}
}
