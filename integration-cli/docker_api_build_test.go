package main

import (
	"archive/tar"
	"bytes"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/request"
	"github.com/docker/docker/pkg/testutil"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestBuildAPIDockerFileRemote(c *check.C) {
	testRequires(c, NotUserNamespace)
	var testD string
	if testEnv.DaemonPlatform() == "windows" {
		testD = `FROM busybox
COPY * /tmp/
RUN find / -name ba*
RUN find /tmp/`
	} else {
		// -xdev is required because sysfs can cause EPERM
		testD = `FROM busybox
COPY * /tmp/
RUN find / -xdev -name ba*
RUN find /tmp/`
	}
	server := fakeStorage(c, map[string]string{"testD": testD})
	defer server.Close()

	res, body, err := request.Post("/build?dockerfile=baz&remote="+server.URL()+"/testD", request.JSON)
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	buf, err := testutil.ReadBody(body)
	c.Assert(err, checker.IsNil)

	// Make sure Dockerfile exists.
	// Make sure 'baz' doesn't exist ANYWHERE despite being mentioned in the URL
	out := string(buf)
	c.Assert(out, checker.Contains, "/tmp/Dockerfile")
	c.Assert(out, checker.Not(checker.Contains), "baz")
}

func (s *DockerSuite) TestBuildAPIRemoteTarballContext(c *check.C) {
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

	server := fakeBinaryStorage(c, map[string]*bytes.Buffer{
		"testT.tar": buffer,
	})
	defer server.Close()

	res, b, err := request.Post("/build?remote="+server.URL()+"/testT.tar", request.ContentType("application/tar"))
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)
	b.Close()
}

func (s *DockerSuite) TestBuildAPIRemoteTarballContextWithCustomDockerfile(c *check.C) {
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

	server := fakeBinaryStorage(c, map[string]*bytes.Buffer{
		"testT.tar": buffer,
	})
	defer server.Close()

	url := "/build?dockerfile=custom&remote=" + server.URL() + "/testT.tar"
	res, body, err := request.Post(url, request.ContentType("application/tar"))
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	defer body.Close()
	content, err := testutil.ReadBody(body)
	c.Assert(err, checker.IsNil)

	// Build used the wrong dockerfile.
	c.Assert(string(content), checker.Not(checker.Contains), "wrong")
}

func (s *DockerSuite) TestBuildAPILowerDockerfile(c *check.C) {
	git := newFakeGit(c, "repo", map[string]string{
		"dockerfile": `FROM busybox
RUN echo from dockerfile`,
	}, false)
	defer git.Close()

	res, body, err := request.Post("/build?remote="+git.RepoURL, request.JSON)
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	buf, err := testutil.ReadBody(body)
	c.Assert(err, checker.IsNil)

	out := string(buf)
	c.Assert(out, checker.Contains, "from dockerfile")
}

func (s *DockerSuite) TestBuildAPIBuildGitWithF(c *check.C) {
	git := newFakeGit(c, "repo", map[string]string{
		"baz": `FROM busybox
RUN echo from baz`,
		"Dockerfile": `FROM busybox
RUN echo from Dockerfile`,
	}, false)
	defer git.Close()

	// Make sure it tries to 'dockerfile' query param value
	res, body, err := request.Post("/build?dockerfile=baz&remote="+git.RepoURL, request.JSON)
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	buf, err := testutil.ReadBody(body)
	c.Assert(err, checker.IsNil)

	out := string(buf)
	c.Assert(out, checker.Contains, "from baz")
}

func (s *DockerSuite) TestBuildAPIDoubleDockerfile(c *check.C) {
	testRequires(c, UnixCli) // dockerfile overwrites Dockerfile on Windows
	git := newFakeGit(c, "repo", map[string]string{
		"Dockerfile": `FROM busybox
RUN echo from Dockerfile`,
		"dockerfile": `FROM busybox
RUN echo from dockerfile`,
	}, false)
	defer git.Close()

	// Make sure it tries to 'dockerfile' query param value
	res, body, err := request.Post("/build?remote="+git.RepoURL, request.JSON)
	c.Assert(err, checker.IsNil)
	c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

	buf, err := testutil.ReadBody(body)
	c.Assert(err, checker.IsNil)

	out := string(buf)
	c.Assert(out, checker.Contains, "from Dockerfile")
}

func (s *DockerSuite) TestBuildAPIUnnormalizedTarPaths(c *check.C) {
	// Make sure that build context tars with entries of the form
	// x/./y don't cause caching false positives.

	buildFromTarContext := func(fileContents []byte) string {
		buffer := new(bytes.Buffer)
		tw := tar.NewWriter(buffer)
		defer tw.Close()

		dockerfile := []byte(`FROM busybox
	COPY dir /dir/`)
		err := tw.WriteHeader(&tar.Header{
			Name: "Dockerfile",
			Size: int64(len(dockerfile)),
		})
		//failed to write tar file header
		c.Assert(err, checker.IsNil)

		_, err = tw.Write(dockerfile)
		// failed to write Dockerfile in tar file content
		c.Assert(err, checker.IsNil)

		err = tw.WriteHeader(&tar.Header{
			Name: "dir/./file",
			Size: int64(len(fileContents)),
		})
		//failed to write tar file header
		c.Assert(err, checker.IsNil)

		_, err = tw.Write(fileContents)
		// failed to write file contents in tar file content
		c.Assert(err, checker.IsNil)

		// failed to close tar archive
		c.Assert(tw.Close(), checker.IsNil)

		res, body, err := request.Post("/build", request.RawContent(ioutil.NopCloser(buffer)), request.ContentType("application/x-tar"))
		c.Assert(err, checker.IsNil)
		c.Assert(res.StatusCode, checker.Equals, http.StatusOK)

		out, err := testutil.ReadBody(body)
		c.Assert(err, checker.IsNil)
		lines := strings.Split(string(out), "\n")
		c.Assert(len(lines), checker.GreaterThan, 1)
		c.Assert(lines[len(lines)-2], checker.Matches, ".*Successfully built [0-9a-f]{12}.*")

		re := regexp.MustCompile("Successfully built ([0-9a-f]{12})")
		matches := re.FindStringSubmatch(lines[len(lines)-2])
		return matches[1]
	}

	imageA := buildFromTarContext([]byte("abc"))
	imageB := buildFromTarContext([]byte("def"))

	c.Assert(imageA, checker.Not(checker.Equals), imageB)
}
