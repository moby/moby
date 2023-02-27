package remotecontext // import "github.com/docker/docker/builder/remotecontext"

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/docker/docker/builder"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/fs"
)

var binaryContext = []byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00} // xz magic

func TestSelectAcceptableMIME(t *testing.T) {
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
			t.Fatalf("Should not have accepted %q", m)
		}
	}

	for _, m := range validMimeStrings {
		if str := selectAcceptableMIME(m); str == "" {
			t.Fatalf("Should have accepted %q", m)
		}
	}
}

func TestInspectEmptyResponse(t *testing.T) {
	ct := "application/octet-stream"
	br := io.NopCloser(bytes.NewReader([]byte("")))
	contentType, bReader, err := inspectResponse(ct, br, 0)
	if err == nil {
		t.Fatal("Should have generated an error for an empty response")
	}
	if contentType != "application/octet-stream" {
		t.Fatalf("Content type should be 'application/octet-stream' but is %q", contentType)
	}
	body, err := io.ReadAll(bReader)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 0 {
		t.Fatal("response body should remain empty")
	}
}

func TestInspectResponseBinary(t *testing.T) {
	ct := "application/octet-stream"
	br := io.NopCloser(bytes.NewReader(binaryContext))
	contentType, bReader, err := inspectResponse(ct, br, int64(len(binaryContext)))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "application/octet-stream" {
		t.Fatalf("Content type should be 'application/octet-stream' but is %q", contentType)
	}
	body, err := io.ReadAll(bReader)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != len(binaryContext) {
		t.Fatalf("Wrong response size %d, should be == len(binaryContext)", len(body))
	}
	for i := range body {
		if body[i] != binaryContext[i] {
			t.Fatalf("Corrupted response body at byte index %d", i)
		}
	}
}

func TestResponseUnsupportedContentType(t *testing.T) {
	content := []byte(dockerfileContents)
	ct := "application/json"
	br := io.NopCloser(bytes.NewReader(content))
	contentType, bReader, err := inspectResponse(ct, br, int64(len(dockerfileContents)))

	if err == nil {
		t.Fatal("Should have returned an error on content-type 'application/json'")
	}
	if contentType != ct {
		t.Fatalf("Should not have altered content-type: orig: %s, altered: %s", ct, contentType)
	}
	body, err := io.ReadAll(bReader)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != dockerfileContents {
		t.Fatalf("Corrupted response body %s", body)
	}
}

func TestInspectResponseTextSimple(t *testing.T) {
	content := []byte(dockerfileContents)
	ct := "text/plain"
	br := io.NopCloser(bytes.NewReader(content))
	contentType, bReader, err := inspectResponse(ct, br, int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "text/plain" {
		t.Fatalf("Content type should be 'text/plain' but is %q", contentType)
	}
	body, err := io.ReadAll(bReader)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != dockerfileContents {
		t.Fatalf("Corrupted response body %s", body)
	}
}

func TestInspectResponseEmptyContentType(t *testing.T) {
	content := []byte(dockerfileContents)
	br := io.NopCloser(bytes.NewReader(content))
	contentType, bodyReader, err := inspectResponse("", br, int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "text/plain" {
		t.Fatalf("Content type should be 'text/plain' but is %q", contentType)
	}
	body, err := io.ReadAll(bodyReader)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != dockerfileContents {
		t.Fatalf("Corrupted response body %s", body)
	}
}

func TestUnknownContentLength(t *testing.T) {
	content := []byte(dockerfileContents)
	ct := "text/plain"
	br := io.NopCloser(bytes.NewReader(content))
	contentType, bReader, err := inspectResponse(ct, br, -1)
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "text/plain" {
		t.Fatalf("Content type should be 'text/plain' but is %q", contentType)
	}
	body, err := io.ReadAll(bReader)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != dockerfileContents {
		t.Fatalf("Corrupted response body %s", body)
	}
}

func TestDownloadRemote(t *testing.T) {
	contextDir := fs.NewDir(t, "test-builder-download-remote",
		fs.WithFile(builder.DefaultDockerfileName, dockerfileContents))
	defer contextDir.Remove()

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	serverURL, _ := url.Parse(server.URL)

	serverURL.Path = "/" + builder.DefaultDockerfileName
	remoteURL := serverURL.String()

	mux.Handle("/", http.FileServer(http.Dir(contextDir.Path())))

	contentType, content, err := downloadRemote(remoteURL)
	assert.NilError(t, err)

	assert.Check(t, is.Equal(mimeTypes.TextPlain, contentType))
	raw, err := io.ReadAll(content)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(dockerfileContents, string(raw)))
}

func TestGetWithStatusError(t *testing.T) {
	var testcases = []struct {
		err          error
		statusCode   int
		expectedErr  string
		expectedBody string
	}{
		{
			statusCode:   200,
			expectedBody: "THE BODY",
		},
		{
			statusCode:   400,
			expectedErr:  "with status 400 Bad Request: broke",
			expectedBody: "broke",
		},
	}
	for _, testcase := range testcases {
		ts := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				buffer := bytes.NewBufferString(testcase.expectedBody)
				w.WriteHeader(testcase.statusCode)
				w.Write(buffer.Bytes())
			}),
		)
		defer ts.Close()
		response, err := GetWithStatusError(ts.URL)

		if testcase.expectedErr == "" {
			assert.NilError(t, err)

			body, err := readBody(response.Body)
			assert.NilError(t, err)
			assert.Check(t, is.Contains(string(body), testcase.expectedBody))
		} else {
			assert.Check(t, is.ErrorContains(err, testcase.expectedErr))
		}
	}
}

func readBody(b io.ReadCloser) ([]byte, error) {
	defer b.Close()
	return io.ReadAll(b)
}
