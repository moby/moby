package builder

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/httputils"
	"github.com/go-check/check"
)

var binaryContext = []byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00} //xz magic

func (s *DockerSuite) TestSelectAcceptableMIME(c *check.C) {
	validMimeStrings := []string{
		"application/x-bzip2",
		"application/bzip2",
		"application/gzip",
		"application/x-gzip",
		"application/x-xz",
		"application/xz",
		"application/tar",
		"application/x-tar",
		"application/octet-stream",
		"text/plain",
	}

	invalidMimeStrings := []string{
		"",
		"application/octet",
		"application/json",
	}

	for _, m := range invalidMimeStrings {
		if len(selectAcceptableMIME(m)) > 0 {
			c.Fatalf("Should not have accepted %q", m)
		}
	}

	for _, m := range validMimeStrings {
		if str := selectAcceptableMIME(m); str == "" {
			c.Fatalf("Should have accepted %q", m)
		}
	}
}

func (s *DockerSuite) TestInspectEmptyResponse(c *check.C) {
	ct := "application/octet-stream"
	br := ioutil.NopCloser(bytes.NewReader([]byte("")))
	contentType, bReader, err := inspectResponse(ct, br, 0)
	if err == nil {
		c.Fatalf("Should have generated an error for an empty response")
	}
	if contentType != "application/octet-stream" {
		c.Fatalf("Content type should be 'application/octet-stream' but is %q", contentType)
	}
	body, err := ioutil.ReadAll(bReader)
	if err != nil {
		c.Fatal(err)
	}
	if len(body) != 0 {
		c.Fatal("response body should remain empty")
	}
}

func (s *DockerSuite) TestInspectResponseBinary(c *check.C) {
	ct := "application/octet-stream"
	br := ioutil.NopCloser(bytes.NewReader(binaryContext))
	contentType, bReader, err := inspectResponse(ct, br, int64(len(binaryContext)))
	if err != nil {
		c.Fatal(err)
	}
	if contentType != "application/octet-stream" {
		c.Fatalf("Content type should be 'application/octet-stream' but is %q", contentType)
	}
	body, err := ioutil.ReadAll(bReader)
	if err != nil {
		c.Fatal(err)
	}
	if len(body) != len(binaryContext) {
		c.Fatalf("Wrong response size %d, should be == len(binaryContext)", len(body))
	}
	for i := range body {
		if body[i] != binaryContext[i] {
			c.Fatalf("Corrupted response body at byte index %d", i)
		}
	}
}

func (s *DockerSuite) TestResponseUnsupportedContentType(c *check.C) {
	content := []byte(dockerfileContents)
	ct := "application/json"
	br := ioutil.NopCloser(bytes.NewReader(content))
	contentType, bReader, err := inspectResponse(ct, br, int64(len(dockerfileContents)))

	if err == nil {
		c.Fatal("Should have returned an error on content-type 'application/json'")
	}
	if contentType != ct {
		c.Fatalf("Should not have altered content-type: orig: %s, altered: %s", ct, contentType)
	}
	body, err := ioutil.ReadAll(bReader)
	if err != nil {
		c.Fatal(err)
	}
	if string(body) != dockerfileContents {
		c.Fatalf("Corrupted response body %s", body)
	}
}

func (s *DockerSuite) TestInspectResponseTextSimple(c *check.C) {
	content := []byte(dockerfileContents)
	ct := "text/plain"
	br := ioutil.NopCloser(bytes.NewReader(content))
	contentType, bReader, err := inspectResponse(ct, br, int64(len(content)))
	if err != nil {
		c.Fatal(err)
	}
	if contentType != "text/plain" {
		c.Fatalf("Content type should be 'text/plain' but is %q", contentType)
	}
	body, err := ioutil.ReadAll(bReader)
	if err != nil {
		c.Fatal(err)
	}
	if string(body) != dockerfileContents {
		c.Fatalf("Corrupted response body %s", body)
	}
}

func (s *DockerSuite) TestInspectResponseEmptyContentType(c *check.C) {
	content := []byte(dockerfileContents)
	br := ioutil.NopCloser(bytes.NewReader(content))
	contentType, bodyReader, err := inspectResponse("", br, int64(len(content)))
	if err != nil {
		c.Fatal(err)
	}
	if contentType != "text/plain" {
		c.Fatalf("Content type should be 'text/plain' but is %q", contentType)
	}
	body, err := ioutil.ReadAll(bodyReader)
	if err != nil {
		c.Fatal(err)
	}
	if string(body) != dockerfileContents {
		c.Fatalf("Corrupted response body %s", body)
	}
}

func (s *DockerSuite) TestMakeRemoteContext(c *check.C) {
	contextDir, cleanup := createTestTempDir(c, "", "builder-tarsum-test")
	defer cleanup()

	createTestTempFile(c, contextDir, DefaultDockerfileName, dockerfileContents, 0777)

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	serverURL, _ := url.Parse(server.URL)

	serverURL.Path = "/" + DefaultDockerfileName
	remoteURL := serverURL.String()

	mux.Handle("/", http.FileServer(http.Dir(contextDir)))

	remoteContext, err := MakeRemoteContext(remoteURL, map[string]func(io.ReadCloser) (io.ReadCloser, error){
		httputils.MimeTypes.TextPlain: func(rc io.ReadCloser) (io.ReadCloser, error) {
			dockerfile, err := ioutil.ReadAll(rc)
			if err != nil {
				return nil, err
			}
			return archive.Generate(DefaultDockerfileName, string(dockerfile))
		},
	})

	if err != nil {
		c.Fatalf("Error when executing DetectContextFromRemoteURL: %s", err)
	}

	if remoteContext == nil {
		c.Fatalf("Remote context should not be nil")
	}

	tarSumCtx, ok := remoteContext.(*tarSumContext)

	if !ok {
		c.Fatalf("Cast error, remote context should be casted to tarSumContext")
	}

	fileInfoSums := tarSumCtx.sums

	if fileInfoSums.Len() != 1 {
		c.Fatalf("Size of file info sums should be 1, got: %d", fileInfoSums.Len())
	}

	fileInfo := fileInfoSums.GetFile(DefaultDockerfileName)

	if fileInfo == nil {
		c.Fatalf("There should be file named %s in fileInfoSums", DefaultDockerfileName)
	}

	if fileInfo.Pos() != 0 {
		c.Fatalf("File %s should have position 0, got %d", DefaultDockerfileName, fileInfo.Pos())
	}
}
