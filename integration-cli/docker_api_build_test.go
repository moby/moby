package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/fakecontext"
	"github.com/moby/moby/v2/internal/testutil/fakegit"
	"github.com/moby/moby/v2/internal/testutil/fakestorage"
	"github.com/moby/moby/v2/internal/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func (s *DockerAPISuite) TestBuildAPIDockerFileRemote(c *testing.T) {
	testRequires(c, NotUserNamespace)
	ctx := testutil.GetContext(c)

	// -xdev is required because sysfs can cause EPERM
	testD := `FROM busybox
RUN find / -xdev -name ba*
RUN find /tmp/`
	server := fakestorage.New(c, "", fakecontext.WithFiles(map[string]string{"testD": testD}))
	defer server.Close()

	res, body, err := request.Post(ctx, "/build?dockerfile=baz&remote="+server.URL()+"/testD", request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusOK)

	buf, err := request.ReadBody(body)
	assert.NilError(c, err)

	// Make sure Dockerfile exists.
	// Make sure 'baz' doesn't exist ANYWHERE despite being mentioned in the URL
	out := string(buf)
	assert.Assert(c, is.Contains(out, "RUN find /tmp"))
	assert.Assert(c, !strings.Contains(out, "baz"))
}

func (s *DockerAPISuite) TestBuildAPIRemoteTarballContext(c *testing.T) {
	ctx := testutil.GetContext(c)

	buffer := new(bytes.Buffer)
	tw := tar.NewWriter(buffer)
	defer tw.Close()

	dockerfile := []byte("FROM busybox")
	err := tw.WriteHeader(&tar.Header{
		Name: "Dockerfile",
		Size: int64(len(dockerfile)),
	})
	assert.NilError(c, err, "failed to write tar file header")

	_, err = tw.Write(dockerfile)
	assert.NilError(c, err, "failed to write tar file content")
	assert.NilError(c, tw.Close(), "failed to close tar archive")

	server := fakestorage.New(c, "", fakecontext.WithBinaryFiles(map[string]*bytes.Buffer{
		"testT.tar": buffer,
	}))
	defer server.Close()

	res, b, err := request.Post(ctx, "/build?remote="+server.URL()+"/testT.tar", request.ContentType("application/tar"))
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusOK)
	b.Close()
}

func (s *DockerAPISuite) TestBuildAPIRemoteTarballContextWithCustomDockerfile(c *testing.T) {
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
	assert.NilError(c, err)

	_, err = tw.Write(dockerfile)
	// failed to write tar file content
	assert.NilError(c, err)

	custom := []byte(`FROM busybox
RUN echo 'right'
`)
	err = tw.WriteHeader(&tar.Header{
		Name: "custom",
		Size: int64(len(custom)),
	})

	// failed to write tar file header
	assert.NilError(c, err)

	_, err = tw.Write(custom)
	// failed to write tar file content
	assert.NilError(c, err)

	// failed to close tar archive
	assert.NilError(c, tw.Close())

	server := fakestorage.New(c, "", fakecontext.WithBinaryFiles(map[string]*bytes.Buffer{
		"testT.tar": buffer,
	}))
	defer server.Close()

	ctx := testutil.GetContext(c)
	url := "/build?dockerfile=custom&remote=" + server.URL() + "/testT.tar"
	res, body, err := request.Post(ctx, url, request.ContentType("application/tar"))
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusOK)

	content, err := request.ReadBody(body)
	assert.NilError(c, err)

	// Build used the wrong dockerfile.
	assert.Assert(c, !strings.Contains(string(content), "wrong"))
}

func (s *DockerAPISuite) TestBuildAPILowerDockerfile(c *testing.T) {
	git := fakegit.New(c, "repo", map[string]string{
		"dockerfile": `FROM busybox
RUN echo from dockerfile`,
	}, false)
	defer git.Close()

	ctx := testutil.GetContext(c)
	res, body, err := request.Post(ctx, "/build?remote="+git.RepoURL, request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusOK)

	buf, err := request.ReadBody(body)
	assert.NilError(c, err)

	out := string(buf)
	assert.Assert(c, is.Contains(out, "from dockerfile"))
}

func (s *DockerAPISuite) TestBuildAPIBuildGitWithF(c *testing.T) {
	git := fakegit.New(c, "repo", map[string]string{
		"baz": `FROM busybox
RUN echo from baz`,
		"Dockerfile": `FROM busybox
RUN echo from Dockerfile`,
	}, false)
	defer git.Close()

	ctx := testutil.GetContext(c)
	// Make sure it tries to 'dockerfile' query param value
	res, body, err := request.Post(ctx, "/build?dockerfile=baz&remote="+git.RepoURL, request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusOK)

	buf, err := request.ReadBody(body)
	assert.NilError(c, err)

	out := string(buf)
	assert.Assert(c, is.Contains(out, "from baz"))
}

func (s *DockerAPISuite) TestBuildAPIDoubleDockerfile(c *testing.T) {
	testRequires(c, UnixCli) // dockerfile overwrites Dockerfile on Windows
	git := fakegit.New(c, "repo", map[string]string{
		"Dockerfile": `FROM busybox
RUN echo from Dockerfile`,
		"dockerfile": `FROM busybox
RUN echo from dockerfile`,
	}, false)
	defer git.Close()

	ctx := testutil.GetContext(c)

	// Make sure it tries to 'dockerfile' query param value
	res, body, err := request.Post(ctx, "/build?remote="+git.RepoURL, request.JSON)
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusOK)

	buf, err := request.ReadBody(body)
	assert.NilError(c, err)

	out := string(buf)
	assert.Assert(c, is.Contains(out, "from Dockerfile"))
}

func (s *DockerAPISuite) TestBuildAPIUnnormalizedTarPaths(c *testing.T) {
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
		assert.NilError(c, err, "failed to write tar file header")

		_, err = tw.Write(dockerfile)
		assert.NilError(c, err, "failed to write Dockerfile in tar file content")

		err = tw.WriteHeader(&tar.Header{
			Name: "dir/./file",
			Size: int64(len(fileContents)),
		})
		assert.NilError(c, err, "failed to write tar file header")

		_, err = tw.Write(fileContents)
		assert.NilError(c, err, "failed to write file contents in tar file content")

		assert.NilError(c, tw.Close(), "failed to close tar archive")

		ctx := testutil.GetContext(c)

		res, body, err := request.Post(ctx, "/build", request.RawContent(io.NopCloser(buffer)), request.ContentType("application/x-tar"))
		assert.NilError(c, err)
		assert.Equal(c, res.StatusCode, http.StatusOK)

		out, err := request.ReadBody(body)
		assert.NilError(c, err)
		lines := strings.Split(string(out), "\n")
		assert.Assert(c, len(lines) > 1)
		matched, err := regexp.MatchString(".*Successfully built [0-9a-f]{12}.*", lines[len(lines)-2])
		assert.NilError(c, err)
		assert.Assert(c, matched)

		re := regexp.MustCompile("Successfully built ([0-9a-f]{12})")
		matches := re.FindStringSubmatch(lines[len(lines)-2])
		return matches[1]
	}

	imageA := buildFromTarContext([]byte("abc"))
	imageB := buildFromTarContext([]byte("def"))

	assert.Assert(c, imageA != imageB)
}

func (s *DockerAPISuite) TestBuildOnBuildWithCopy(c *testing.T) {
	dockerfile := `
		FROM ` + minimalBaseImage() + ` as onbuildbase
		ONBUILD COPY file /file

		FROM onbuildbase
	`
	bCtx := fakecontext.New(c, "",
		fakecontext.WithDockerfile(dockerfile),
		fakecontext.WithFile("file", "some content"),
	)

	ctx := testutil.GetContext(c)
	res, body, err := request.Post(
		ctx,
		"/build",
		request.RawContent(bCtx.AsTarReader(c)),
		request.ContentType("application/x-tar"))
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusOK)

	out, err := request.ReadBody(body)
	assert.NilError(c, err)
	assert.Assert(c, is.Contains(string(out), "Successfully built"))
}

func (s *DockerAPISuite) TestBuildOnBuildCache(c *testing.T) {
	build := func(dockerfile string) []byte {
		bCtx := fakecontext.New(c, "",
			fakecontext.WithDockerfile(dockerfile),
		)

		ctx := testutil.GetContext(c)
		res, body, err := request.Post(
			ctx,
			"/build",
			request.RawContent(bCtx.AsTarReader(c)),
			request.ContentType("application/x-tar"))
		assert.NilError(c, err)
		assert.Check(c, is.DeepEqual(http.StatusOK, res.StatusCode))

		out, err := request.ReadBody(body)
		assert.NilError(c, err)
		assert.Assert(c, is.Contains(string(out), "Successfully built"))
		return out
	}

	dockerfile := `
		FROM ` + minimalBaseImage() + ` as onbuildbase
		ENV something=bar
		ONBUILD ENV foo=bar
	`
	build(dockerfile)

	dockerfile += "FROM onbuildbase"
	out := build(dockerfile)

	imageIDs := getImageIDsFromBuild(c, out)
	assert.Assert(c, is.Len(imageIDs, 2))
	parentID, childID := imageIDs[0], imageIDs[1]

	apiClient := testEnv.APIClient()
	ctx := testutil.GetContext(c)

	// check parentID is correct
	// Parent is graphdriver-only
	if !testEnv.UsingSnapshotter() {
		var buf bytes.Buffer
		_, err := apiClient.ImageInspect(ctx, childID, client.ImageInspectWithRawResponse(&buf))
		assert.NilError(c, err)

		var image struct {
			// Parent is the ID of the parent image.
			//
			// Depending on how the image was created, this field may be empty and
			// is only set for images that were built/created locally. This field
			// is omitted if the image was pulled from an image registry.
			Parent string `json:",omitempty"`
		}
		rawResponse := buf.Bytes()
		err = json.Unmarshal(rawResponse, &image)
		assert.NilError(c, err, string(rawResponse))
		assert.Check(c, is.Equal(parentID, image.Parent), string(rawResponse))
	}
}

func (s *DockerRegistrySuite) TestBuildCopyFromForcePull(c *testing.T) {
	apiClient := testEnv.APIClient()

	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)
	// tag the image to upload it to the private registry
	ctx := testutil.GetContext(c)
	_, err := apiClient.ImageTag(ctx, client.ImageTagOptions{Source: "busybox", Target: repoName})
	assert.Check(c, err)
	// push the image to the registry
	rc, err := apiClient.ImagePush(ctx, repoName, client.ImagePushOptions{RegistryAuth: "{}"})
	assert.Check(c, err)
	_, err = io.Copy(io.Discard, rc)
	assert.Check(c, err)

	dockerfile := fmt.Sprintf(`
		FROM %s AS foo
		RUN touch abc
		FROM %s
		COPY --from=foo /abc /
		`, repoName, repoName)

	bCtx := fakecontext.New(c, "",
		fakecontext.WithDockerfile(dockerfile),
	)

	res, body, err := request.Post(
		ctx,
		"/build?pull=1",
		request.RawContent(bCtx.AsTarReader(c)),
		request.ContentType("application/x-tar"))
	assert.NilError(c, err)
	assert.Check(c, is.DeepEqual(http.StatusOK, res.StatusCode))

	out, err := request.ReadBody(body)
	assert.NilError(c, err)
	assert.Check(c, is.Contains(string(out), "Successfully built"))
}

func (s *DockerAPISuite) TestBuildAddRemoteNoDecompress(c *testing.T) {
	buffer := new(bytes.Buffer)
	tw := tar.NewWriter(buffer)
	dt := []byte("contents")
	err := tw.WriteHeader(&tar.Header{
		Name:     "foo",
		Size:     int64(len(dt)),
		Mode:     0o600,
		Typeflag: tar.TypeReg,
	})
	assert.NilError(c, err)
	_, err = tw.Write(dt)
	assert.NilError(c, err)
	err = tw.Close()
	assert.NilError(c, err)

	server := fakestorage.New(c, "", fakecontext.WithBinaryFiles(map[string]*bytes.Buffer{
		"test.tar": buffer,
	}))
	defer server.Close()

	dockerfile := fmt.Sprintf(`
		FROM busybox
		ADD %s/test.tar /
		RUN [ -f test.tar ]
		`, server.URL())

	bCtx := fakecontext.New(c, "",
		fakecontext.WithDockerfile(dockerfile),
	)

	ctx := testutil.GetContext(c)
	res, body, err := request.Post(
		ctx,
		"/build",
		request.RawContent(bCtx.AsTarReader(c)),
		request.ContentType("application/x-tar"))
	assert.NilError(c, err)
	assert.Check(c, is.DeepEqual(http.StatusOK, res.StatusCode))

	out, err := request.ReadBody(body)
	assert.NilError(c, err)
	assert.Check(c, is.Contains(string(out), "Successfully built"))
}

func (s *DockerAPISuite) TestBuildChownOnCopy(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	dockerfile := `FROM busybox
		RUN echo 'test1:x:1001:1001::/bin:/bin/false' >> /etc/passwd
		RUN echo 'test1:x:1001:' >> /etc/group
		RUN echo 'test2:x:1002:' >> /etc/group
		COPY --chown=test1:1002 . /new_dir
		RUN ls -l /
		RUN [ $(ls -l / | grep new_dir | awk '{print $3":"$4}') = 'test1:test2' ]
		RUN [ $(ls -nl / | grep new_dir | awk '{print $3":"$4}') = '1001:1002' ]
	`
	bCtx := fakecontext.New(c, "",
		fakecontext.WithDockerfile(dockerfile),
		fakecontext.WithFile("test_file1", "some test content"),
	)

	ctx := testutil.GetContext(c)
	res, body, err := request.Post(
		ctx,
		"/build",
		request.RawContent(bCtx.AsTarReader(c)),
		request.ContentType("application/x-tar"))
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusOK)

	out, err := request.ReadBody(body)
	assert.NilError(c, err)
	assert.Check(c, is.Contains(string(out), "Successfully built"))
}

func (s *DockerAPISuite) TestBuildCopyCacheOnFileChange(c *testing.T) {
	dockerfile := `FROM busybox
COPY file /file`

	ctx1 := fakecontext.New(c, "",
		fakecontext.WithDockerfile(dockerfile),
		fakecontext.WithFile("file", "foo"))
	ctx2 := fakecontext.New(c, "",
		fakecontext.WithDockerfile(dockerfile),
		fakecontext.WithFile("file", "bar"))

	ctx := testutil.GetContext(c)
	build := func(bCtx *fakecontext.Fake) string {
		res, body, err := request.Post(ctx, "/build",
			request.RawContent(bCtx.AsTarReader(c)),
			request.ContentType("application/x-tar"))

		assert.NilError(c, err)
		assert.Check(c, is.DeepEqual(http.StatusOK, res.StatusCode))

		out, err := request.ReadBody(body)
		assert.NilError(c, err)
		assert.Assert(c, is.Contains(string(out), "Successfully built"))

		ids := getImageIDsFromBuild(c, out)
		assert.Assert(c, is.Len(ids, 1))
		return ids[len(ids)-1]
	}

	id1 := build(ctx1)
	id2 := build(ctx1)
	id3 := build(ctx2)

	if id1 != id2 {
		c.Fatal("didn't use the cache")
	}
	if id1 == id3 {
		c.Fatal("COPY With different source file should not share same cache")
	}
}

func (s *DockerAPISuite) TestBuildAddCacheOnFileChange(c *testing.T) {
	dockerfile := `FROM busybox
ADD file /file`

	ctx1 := fakecontext.New(c, "",
		fakecontext.WithDockerfile(dockerfile),
		fakecontext.WithFile("file", "foo"))
	ctx2 := fakecontext.New(c, "",
		fakecontext.WithDockerfile(dockerfile),
		fakecontext.WithFile("file", "bar"))

	ctx := testutil.GetContext(c)
	build := func(bCtx *fakecontext.Fake) string {
		res, body, err := request.Post(ctx, "/build",
			request.RawContent(bCtx.AsTarReader(c)),
			request.ContentType("application/x-tar"))

		assert.NilError(c, err)
		assert.Check(c, is.DeepEqual(http.StatusOK, res.StatusCode))

		out, err := request.ReadBody(body)
		assert.NilError(c, err)
		assert.Assert(c, is.Contains(string(out), "Successfully built"))

		ids := getImageIDsFromBuild(c, out)
		assert.Assert(c, is.Len(ids, 1))
		return ids[len(ids)-1]
	}

	id1 := build(ctx1)
	id2 := build(ctx1)
	id3 := build(ctx2)

	if id1 != id2 {
		c.Fatal("didn't use the cache")
	}
	if id1 == id3 {
		c.Fatal("COPY With different source file should not share same cache")
	}
}

func (s *DockerAPISuite) TestBuildScratchCopy(c *testing.T) {
	testRequires(c, DaemonIsLinux)
	dockerfile := `FROM scratch
ADD Dockerfile /
ENV foo bar`
	bCtx := fakecontext.New(c, "",
		fakecontext.WithDockerfile(dockerfile),
	)

	ctx := testutil.GetContext(c)
	res, body, err := request.Post(
		ctx,
		"/build",
		request.RawContent(bCtx.AsTarReader(c)),
		request.ContentType("application/x-tar"))
	assert.NilError(c, err)
	assert.Equal(c, res.StatusCode, http.StatusOK)

	out, err := request.ReadBody(body)
	assert.NilError(c, err)
	assert.Check(c, is.Contains(string(out), "Successfully built"))
}

type buildLine struct {
	Stream string
	Aux    struct {
		ID string
	}
}

func getImageIDsFromBuild(t *testing.T, output []byte) []string {
	var ids []string
	for line := range bytes.SplitSeq(output, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		entry := buildLine{}
		assert.NilError(t, json.Unmarshal(line, &entry))
		if entry.Aux.ID != "" {
			ids = append(ids, entry.Aux.ID)
		}
	}
	return ids
}
