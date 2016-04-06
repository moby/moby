package main

import (
	"archive/tar"
	"bytes"
	"net/http"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestBuildApiDockerfilePath(c *check.C) {
	// Test to make sure we stop people from trying to leave the
	// build context when specifying the path to the dockerfile
	buffer := new(bytes.Buffer)
	tw := tar.NewWriter(buffer)
	defer tw.Close()

	dockerfile := []byte("FROM busybox")
	err := tw.WriteHeader(&tar.Header{
		Name: "Dockerfile",
		Size: int64(len(dockerfile)),
	})
	//failed to write tar file header
	c.Assert(err, checker.IsNil)

	_, err = tw.Write(dockerfile)
	// failed to write tar file content
	c.Assert(err, checker.IsNil)

	// failed to close tar archive
	c.Assert(tw.Close(), checker.IsNil)

	res, body, err := sockRequestRaw("POST", "/build?dockerfile=../Dockerfile", buffer, "application/x-tar")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusInternalServerError)

	out, err := readBody(body)
	c.Assert(err, checker.IsNil)

	// Didn't complain about leaving build context
	c.Assert(string(out), checker.Contains, "Forbidden path outside the build context")
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
	c.Assert(err, checker.IsNil)
	defer server.Close()

	res, body, err := sockRequestRaw("POST", "/build?dockerfile=baz&remote="+server.URL()+"/testD", nil, "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	buf, err := readBody(body)
	c.Assert(err, checker.IsNil)

	// Make sure Dockerfile exists.
	// Make sure 'baz' doesn't exist ANYWHERE despite being mentioned in the URL
	out := string(buf)
	c.Assert(out, checker.Contains, "/tmp/Dockerfile")
	c.Assert(out, checker.Not(checker.Contains), "baz")
}

func (s *DockerSuite) TestBuildApiRemoteTarballContext(c *check.C) {
	testRequires(c, DaemonIsLinux)
	buffer := new(bytes.Buffer)
	tw := tar.NewWriter(buffer)
	defer tw.Close()

	dockerfile := []byte("FROM busybox")
	err := tw.WriteHeader(&tar.Header{
		Name: "Dockerfile",
		Size: int64(len(dockerfile)),
	})
	// failed to write tar file header
	c.Assert(err, checker.IsNil)

	_, err = tw.Write(dockerfile)
	// failed to write tar file content
	c.Assert(err, checker.IsNil)

	// failed to close tar archive
	c.Assert(tw.Close(), checker.IsNil)

	server, err := fakeBinaryStorage(map[string]*bytes.Buffer{
		"testT.tar": buffer,
	})
	c.Assert(err, checker.IsNil)

	defer server.Close()

	res, b, err := sockRequestRaw("POST", "/build?remote="+server.URL()+"/testT.tar", nil, "application/tar")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)
	b.Close()
}

func (s *DockerSuite) TestBuildApiRemoteTarballContextWithCustomDockerfile(c *check.C) {
	testRequires(c, DaemonIsLinux)
	buffer := new(bytes.Buffer)
	tw := tar.NewWriter(buffer)
	defer tw.Close()

	dockerfile := []byte(`FROM busybox
RUN echo 'wrong'`)
	err := tw.WriteHeader(&tar.Header{
		Name: "Dockerfile",
		Size: int64(len(dockerfile)),
	})
	// failed to write tar file header
	c.Assert(err, checker.IsNil)

	_, err = tw.Write(dockerfile)
	// failed to write tar file content
	c.Assert(err, checker.IsNil)

	custom := []byte(`FROM busybox
RUN echo 'right'
`)
	err = tw.WriteHeader(&tar.Header{
		Name: "custom",
		Size: int64(len(custom)),
	})

	// failed to write tar file header
	c.Assert(err, checker.IsNil)

	_, err = tw.Write(custom)
	// failed to write tar file content
	c.Assert(err, checker.IsNil)

	// failed to close tar archive
	c.Assert(tw.Close(), checker.IsNil)

	server, err := fakeBinaryStorage(map[string]*bytes.Buffer{
		"testT.tar": buffer,
	})
	c.Assert(err, checker.IsNil)

	defer server.Close()
	url := "/build?dockerfile=custom&remote=" + server.URL() + "/testT.tar"
	res, body, err := sockRequestRaw("POST", url, nil, "application/tar")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	defer body.Close()
	content, err := readBody(body)
	c.Assert(err, checker.IsNil)

	// Build used the wrong dockerfile.
	c.Assert(string(content), checker.Not(checker.Contains), "wrong")
}

func (s *DockerSuite) TestBuildApiLowerDockerfile(c *check.C) {
	testRequires(c, DaemonIsLinux)
	git, err := newFakeGit("repo", map[string]string{
		"dockerfile": `FROM busybox
RUN echo from dockerfile`,
	}, false)
	c.Assert(err, checker.IsNil)
	defer git.Close()

	res, body, err := sockRequestRaw("POST", "/build?remote="+git.RepoURL, nil, "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	buf, err := readBody(body)
	c.Assert(err, checker.IsNil)

	out := string(buf)
	c.Assert(out, checker.Contains, "from dockerfile")
}

func (s *DockerSuite) TestBuildApiBuildGitWithF(c *check.C) {
	testRequires(c, DaemonIsLinux)
	git, err := newFakeGit("repo", map[string]string{
		"baz": `FROM busybox
RUN echo from baz`,
		"Dockerfile": `FROM busybox
RUN echo from Dockerfile`,
	}, false)
	c.Assert(err, checker.IsNil)
	defer git.Close()

	// Make sure it tries to 'dockerfile' query param value
	res, body, err := sockRequestRaw("POST", "/build?dockerfile=baz&remote="+git.RepoURL, nil, "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	buf, err := readBody(body)
	c.Assert(err, checker.IsNil)

	out := string(buf)
	c.Assert(out, checker.Contains, "from baz")
}

func (s *DockerSuite) TestBuildApiDoubleDockerfile(c *check.C) {
	testRequires(c, UnixCli) // dockerfile overwrites Dockerfile on Windows
	git, err := newFakeGit("repo", map[string]string{
		"Dockerfile": `FROM busybox
RUN echo from Dockerfile`,
		"dockerfile": `FROM busybox
RUN echo from dockerfile`,
	}, false)
	c.Assert(err, checker.IsNil)
	defer git.Close()

	// Make sure it tries to 'dockerfile' query param value
	res, body, err := sockRequestRaw("POST", "/build?remote="+git.RepoURL, nil, "application/json")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	buf, err := readBody(body)
	c.Assert(err, checker.IsNil)

	out := string(buf)
	c.Assert(out, checker.Contains, "from Dockerfile")
}

func (s *DockerSuite) TestBuildApiDockerfileSymlink(c *check.C) {
	// Test to make sure we stop people from trying to leave the
	// build context when specifying a symlink as the path to the dockerfile
	buffer := new(bytes.Buffer)
	tw := tar.NewWriter(buffer)
	defer tw.Close()

	err := tw.WriteHeader(&tar.Header{
		Name:     "Dockerfile",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
	})
	// failed to write tar file header
	c.Assert(err, checker.IsNil)

	// failed to close tar archive
	c.Assert(tw.Close(), checker.IsNil)

	res, body, err := sockRequestRaw("POST", "/build", buffer, "application/x-tar")
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusInternalServerError)

	out, err := readBody(body)
	c.Assert(err, checker.IsNil)

	// The reason the error is "Cannot locate specified Dockerfile" is because
	// in the builder, the symlink is resolved within the context, therefore
	// Dockerfile -> /etc/passwd becomes etc/passwd from the context which is
	// a nonexistent file.
	c.Assert(string(out), checker.Contains, "Cannot locate specified Dockerfile: Dockerfile", check.Commentf("Didn't complain about leaving build context"))
}
