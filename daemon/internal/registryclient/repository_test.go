package registryclient

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/distribution/reference"
	"github.com/docker/distribution"
	"github.com/docker/distribution/registry/api/errcode"
	v2 "github.com/docker/distribution/registry/api/v2"
	"github.com/google/uuid"
	"github.com/moby/moby/v2/daemon/internal/registryclient/testutil"
	"github.com/opencontainers/go-digest"
)

func testServer(rrm testutil.RequestResponseMap) (string, func()) {
	h := testutil.NewHandler(rrm)
	s := httptest.NewServer(h)
	return s.URL, s.Close
}

func newRandomBlob(size int) (digest.Digest, []byte) {
	b := make([]byte, size)
	if n, err := rand.Read(b); err != nil {
		panic(err)
	} else if n != size {
		panic("unable to read enough bytes")
	}

	return digest.FromBytes(b), b
}

func addTestFetch(repo string, dgst digest.Digest, content []byte, m *testutil.RequestResponseMap) {
	*m = append(*m, testutil.RequestResponseMapping{
		Request: testutil.Request{
			Method: http.MethodGet,
			Route:  "/v2/" + repo + "/blobs/" + dgst.String(),
		},
		Response: testutil.Response{
			StatusCode: http.StatusOK,
			Body:       content,
			Headers: http.Header{
				"Content-Length": {fmt.Sprint(len(content))},
				"Last-Modified":  {time.Now().Add(-1 * time.Second).Format(time.ANSIC)},
			},
		},
	})

	*m = append(*m, testutil.RequestResponseMapping{
		Request: testutil.Request{
			Method: http.MethodHead,
			Route:  "/v2/" + repo + "/blobs/" + dgst.String(),
		},
		Response: testutil.Response{
			StatusCode: http.StatusOK,
			Headers: http.Header{
				"Content-Length": {fmt.Sprint(len(content))},
				"Last-Modified":  {time.Now().Add(-1 * time.Second).Format(time.ANSIC)},
			},
		},
	})
}

func TestBlobFetch(t *testing.T) {
	d1, b1 := newRandomBlob(1024)
	var m testutil.RequestResponseMap
	addTestFetch("test.example.com/repo1", d1, b1, &m)

	e, c := testServer(m)
	defer c()

	ctx := context.Background()
	repo, _ := reference.WithName("test.example.com/repo1")
	r, err := NewRepository(repo, e, nil)
	if err != nil {
		t.Fatal(err)
	}
	l := r.Blobs(ctx)

	b, err := l.Get(ctx, d1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, b1) {
		t.Fatalf("Wrong bytes values fetched: [%d]byte != [%d]byte", len(b), len(b1))
	}

	// TODO(dmcgowan): Test for unknown blob case
}

func TestBlobExistsNoContentLength(t *testing.T) {
	var m testutil.RequestResponseMap

	repo, _ := reference.WithName("biff")
	dgst, content := newRandomBlob(1024)
	m = append(m, testutil.RequestResponseMapping{
		Request: testutil.Request{
			Method: http.MethodGet,
			Route:  "/v2/" + repo.Name() + "/blobs/" + dgst.String(),
		},
		Response: testutil.Response{
			StatusCode: http.StatusOK,
			Body:       content,
			Headers: http.Header{
				"Last-Modified": {time.Now().Add(-1 * time.Second).Format(time.ANSIC)},
			},
		},
	})

	m = append(m, testutil.RequestResponseMapping{
		Request: testutil.Request{
			Method: http.MethodHead,
			Route:  "/v2/" + repo.Name() + "/blobs/" + dgst.String(),
		},
		Response: testutil.Response{
			StatusCode: http.StatusOK,
			Headers: http.Header{
				"Last-Modified": {time.Now().Add(-1 * time.Second).Format(time.ANSIC)},
			},
		},
	})
	e, c := testServer(m)
	defer c()

	ctx := context.Background()
	r, err := NewRepository(repo, e, nil)
	if err != nil {
		t.Fatal(err)
	}
	l := r.Blobs(ctx)

	_, err = l.Stat(ctx, dgst)
	if err == nil {
		t.Fatal(err)
	}
	if !strings.Contains(err.Error(), "missing content-length heade") {
		t.Fatalf("Expected missing content-length error message")
	}
}

func TestBlobExists(t *testing.T) {
	d1, b1 := newRandomBlob(1024)
	var m testutil.RequestResponseMap
	addTestFetch("test.example.com/repo1", d1, b1, &m)

	e, c := testServer(m)
	defer c()

	ctx := context.Background()
	repo, _ := reference.WithName("test.example.com/repo1")
	r, err := NewRepository(repo, e, nil)
	if err != nil {
		t.Fatal(err)
	}
	l := r.Blobs(ctx)

	stat, err := l.Stat(ctx, d1)
	if err != nil {
		t.Fatal(err)
	}

	if stat.Digest != d1 {
		t.Fatalf("Unexpected digest: %s, expected %s", stat.Digest, d1)
	}

	if stat.Size != int64(len(b1)) {
		t.Fatalf("Unexpected length: %d, expected %d", stat.Size, len(b1))
	}

	// TODO(dmcgowan): Test error cases and ErrBlobUnknown case
}

func TestBlobUploadChunked(t *testing.T) {
	dgst, b1 := newRandomBlob(1024)
	var m testutil.RequestResponseMap
	chunks := [][]byte{
		b1[0:256],
		b1[256:512],
		b1[512:513],
		b1[513:1024],
	}
	repo, _ := reference.WithName("test.example.com/uploadrepo")
	uuids := []string{uuid.NewString()}
	m = append(m, testutil.RequestResponseMapping{
		Request: testutil.Request{
			Method: http.MethodPost,
			Route:  "/v2/" + repo.Name() + "/blobs/uploads/",
		},
		Response: testutil.Response{
			StatusCode: http.StatusAccepted,
			Headers: http.Header{
				"Content-Length":     {"0"},
				"Location":           {"/v2/" + repo.Name() + "/blobs/uploads/" + uuids[0]},
				"Docker-Upload-UUID": {uuids[0]},
				"Range":              {"0-0"},
			},
		},
	})
	offset := 0
	for i, chunk := range chunks {
		uuids = append(uuids, uuid.NewString())
		newOffset := offset + len(chunk)
		m = append(m, testutil.RequestResponseMapping{
			Request: testutil.Request{
				Method: http.MethodPatch,
				Route:  "/v2/" + repo.Name() + "/blobs/uploads/" + uuids[i],
				Body:   chunk,
			},
			Response: testutil.Response{
				StatusCode: http.StatusAccepted,
				Headers: http.Header{
					"Content-Length":     {"0"},
					"Location":           {"/v2/" + repo.Name() + "/blobs/uploads/" + uuids[i+1]},
					"Docker-Upload-UUID": {uuids[i+1]},
					"Range":              {fmt.Sprintf("%d-%d", offset, newOffset-1)},
				},
			},
		})
		offset = newOffset
	}
	m = append(m, testutil.RequestResponseMapping{
		Request: testutil.Request{
			Method: http.MethodPut,
			Route:  "/v2/" + repo.Name() + "/blobs/uploads/" + uuids[len(uuids)-1],
			QueryParams: map[string][]string{
				"digest": {dgst.String()},
			},
		},
		Response: testutil.Response{
			StatusCode: http.StatusCreated,
			Headers: http.Header{
				"Content-Length":        {"0"},
				"Docker-Content-Digest": {dgst.String()},
				"Content-Range":         {fmt.Sprintf("0-%d", offset-1)},
			},
		},
	})
	m = append(m, testutil.RequestResponseMapping{
		Request: testutil.Request{
			Method: http.MethodHead,
			Route:  "/v2/" + repo.Name() + "/blobs/" + dgst.String(),
		},
		Response: testutil.Response{
			StatusCode: http.StatusOK,
			Headers: http.Header{
				"Content-Length": {fmt.Sprint(offset)},
				"Last-Modified":  {time.Now().Add(-1 * time.Second).Format(time.ANSIC)},
			},
		},
	})

	e, c := testServer(m)
	defer c()

	ctx := context.Background()
	r, err := NewRepository(repo, e, nil)
	if err != nil {
		t.Fatal(err)
	}
	l := r.Blobs(ctx)

	upload, err := l.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if upload.ID() != uuids[0] {
		log.Fatalf("Unexpected UUID %s; expected %s", upload.ID(), uuids[0])
	}

	for _, chunk := range chunks {
		n, err := upload.Write(chunk)
		if err != nil {
			t.Fatal(err)
		}
		if n != len(chunk) {
			t.Fatalf("Unexpected length returned from write: %d; expected: %d", n, len(chunk))
		}
	}

	blob, err := upload.Commit(ctx, distribution.Descriptor{
		Digest: dgst,
		Size:   int64(len(b1)),
	})
	if err != nil {
		t.Fatal(err)
	}

	if blob.Size != int64(len(b1)) {
		t.Fatalf("Unexpected blob size: %d; expected: %d", blob.Size, len(b1))
	}
}

func TestBlobUploadMonolithic(t *testing.T) {
	dgst, b1 := newRandomBlob(1024)
	var m testutil.RequestResponseMap
	repo, _ := reference.WithName("test.example.com/uploadrepo")
	uploadID := uuid.NewString()
	m = append(m, testutil.RequestResponseMapping{
		Request: testutil.Request{
			Method: http.MethodPost,
			Route:  "/v2/" + repo.Name() + "/blobs/uploads/",
		},
		Response: testutil.Response{
			StatusCode: http.StatusAccepted,
			Headers: http.Header{
				"Content-Length":     {"0"},
				"Location":           {"/v2/" + repo.Name() + "/blobs/uploads/" + uploadID},
				"Docker-Upload-UUID": {uploadID},
				"Range":              {"0-0"},
			},
		},
	})
	m = append(m, testutil.RequestResponseMapping{
		Request: testutil.Request{
			Method: http.MethodPatch,
			Route:  "/v2/" + repo.Name() + "/blobs/uploads/" + uploadID,
			Body:   b1,
		},
		Response: testutil.Response{
			StatusCode: http.StatusAccepted,
			Headers: http.Header{
				"Location":              {"/v2/" + repo.Name() + "/blobs/uploads/" + uploadID},
				"Docker-Upload-UUID":    {uploadID},
				"Content-Length":        {"0"},
				"Docker-Content-Digest": {dgst.String()},
				"Range":                 {fmt.Sprintf("0-%d", len(b1)-1)},
			},
		},
	})
	m = append(m, testutil.RequestResponseMapping{
		Request: testutil.Request{
			Method: http.MethodPut,
			Route:  "/v2/" + repo.Name() + "/blobs/uploads/" + uploadID,
			QueryParams: map[string][]string{
				"digest": {dgst.String()},
			},
		},
		Response: testutil.Response{
			StatusCode: http.StatusCreated,
			Headers: http.Header{
				"Content-Length":        {"0"},
				"Docker-Content-Digest": {dgst.String()},
				"Content-Range":         {fmt.Sprintf("0-%d", len(b1)-1)},
			},
		},
	})
	m = append(m, testutil.RequestResponseMapping{
		Request: testutil.Request{
			Method: http.MethodHead,
			Route:  "/v2/" + repo.Name() + "/blobs/" + dgst.String(),
		},
		Response: testutil.Response{
			StatusCode: http.StatusOK,
			Headers: http.Header{
				"Content-Length": {fmt.Sprint(len(b1))},
				"Last-Modified":  {time.Now().Add(-1 * time.Second).Format(time.ANSIC)},
			},
		},
	})

	e, c := testServer(m)
	defer c()

	ctx := context.Background()
	r, err := NewRepository(repo, e, nil)
	if err != nil {
		t.Fatal(err)
	}
	l := r.Blobs(ctx)

	upload, err := l.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if upload.ID() != uploadID {
		log.Fatalf("Unexpected UUID %s; expected %s", upload.ID(), uploadID)
	}

	n, err := upload.ReadFrom(bytes.NewReader(b1))
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(b1)) {
		t.Fatalf("Unexpected ReadFrom length: %d; expected: %d", n, len(b1))
	}

	blob, err := upload.Commit(ctx, distribution.Descriptor{
		Digest: dgst,
		Size:   int64(len(b1)),
	})
	if err != nil {
		t.Fatal(err)
	}

	if blob.Size != int64(len(b1)) {
		t.Fatalf("Unexpected blob size: %d; expected: %d", blob.Size, len(b1))
	}
}

func TestBlobMount(t *testing.T) {
	dgst, content := newRandomBlob(1024)
	var m testutil.RequestResponseMap
	repo, _ := reference.WithName("test.example.com/uploadrepo")

	sourceRepo, _ := reference.WithName("test.example.com/sourcerepo")
	canonicalRef, _ := reference.WithDigest(sourceRepo, dgst)

	m = append(m, testutil.RequestResponseMapping{
		Request: testutil.Request{
			Method:      http.MethodPost,
			Route:       "/v2/" + repo.Name() + "/blobs/uploads/",
			QueryParams: map[string][]string{"from": {sourceRepo.Name()}, "mount": {dgst.String()}},
		},
		Response: testutil.Response{
			StatusCode: http.StatusCreated,
			Headers: http.Header{
				"Content-Length":        {"0"},
				"Location":              {"/v2/" + repo.Name() + "/blobs/" + dgst.String()},
				"Docker-Content-Digest": {dgst.String()},
			},
		},
	})
	m = append(m, testutil.RequestResponseMapping{
		Request: testutil.Request{
			Method: http.MethodHead,
			Route:  "/v2/" + repo.Name() + "/blobs/" + dgst.String(),
		},
		Response: testutil.Response{
			StatusCode: http.StatusOK,
			Headers: http.Header{
				"Content-Length": {fmt.Sprint(len(content))},
				"Last-Modified":  {time.Now().Add(-1 * time.Second).Format(time.ANSIC)},
			},
		},
	})

	e, c := testServer(m)
	defer c()

	ctx := context.Background()
	r, err := NewRepository(repo, e, nil)
	if err != nil {
		t.Fatal(err)
	}

	l := r.Blobs(ctx)

	bw, err := l.Create(ctx, WithMountFrom(canonicalRef))
	if bw != nil {
		t.Fatalf("Expected blob writer to be nil, was %v", bw)
	}

	var ebm distribution.ErrBlobMounted
	if errors.As(err, &ebm) {
		if ebm.From.Digest() != dgst {
			t.Fatalf("Unexpected digest: %s, expected %s", ebm.From.Digest(), dgst)
		}
		if ebm.From.Name() != sourceRepo.Name() {
			t.Fatalf("Unexpected from: %s, expected %s", ebm.From.Name(), sourceRepo)
		}
	}
}

func TestManifestTags(t *testing.T) {
	repo, _ := reference.WithName("test.example.com/repo/tags/list")
	tagsList := []byte(strings.TrimSpace(`
{
	"name": "test.example.com/repo/tags/list",
	"tags": [
		"tag1",
		"tag2",
		"funtag"
	]
}
	`))
	var m testutil.RequestResponseMap
	for i := 0; i < 3; i++ {
		m = append(m, testutil.RequestResponseMapping{
			Request: testutil.Request{
				Method: http.MethodGet,
				Route:  "/v2/" + repo.Name() + "/tags/list",
			},
			Response: testutil.Response{
				StatusCode: http.StatusOK,
				Body:       tagsList,
				Headers: http.Header{
					"Content-Length": {fmt.Sprint(len(tagsList))},
					"Last-Modified":  {time.Now().Add(-1 * time.Second).Format(time.ANSIC)},
				},
			},
		})
	}
	e, c := testServer(m)
	defer c()

	r, err := NewRepository(repo, e, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	tagService := r.Tags(ctx)

	allTags, err := tagService.All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(allTags) != 3 {
		t.Fatalf("Wrong number of tags returned: %d, expected 3", len(allTags))
	}

	expected := map[string]struct{}{
		"tag1":   {},
		"tag2":   {},
		"funtag": {},
	}
	for _, t := range allTags {
		delete(expected, t)
	}
	if len(expected) != 0 {
		t.Fatalf("unexpected tags returned: %v", expected)
	}
	// TODO(dmcgowan): Check for error cases
}

func TestObtainsErrorForMissingTag(t *testing.T) {
	repo, _ := reference.WithName("test.example.com/repo")

	var m testutil.RequestResponseMap
	var errs errcode.Errors
	errs = append(errs, v2.ErrorCodeManifestUnknown.WithDetail("unknown manifest"))
	errBytes, err := json.Marshal(errs)
	if err != nil {
		t.Fatal(err)
	}
	m = append(m, testutil.RequestResponseMapping{
		Request: testutil.Request{
			Method: http.MethodGet,
			Route:  "/v2/" + repo.Name() + "/manifests/1.0.0",
		},
		Response: testutil.Response{
			StatusCode: http.StatusNotFound,
			Body:       errBytes,
			Headers: http.Header{
				"Content-Type": {"application/json; charset=utf-8"},
			},
		},
	})
	e, c := testServer(m)
	defer c()

	ctx := context.Background()
	r, err := NewRepository(repo, e, nil)
	if err != nil {
		t.Fatal(err)
	}

	tagService := r.Tags(ctx)

	_, err = tagService.Get(ctx, "1.0.0")
	if err == nil {
		t.Fatalf("Expected an error")
	}
	if !strings.Contains(err.Error(), "manifest unknown") {
		t.Fatalf("Expected unknown manifest error message")
	}
}

func TestManifestTagsPaginated(t *testing.T) {
	s := httptest.NewServer(http.NotFoundHandler())
	defer s.Close()

	repo, _ := reference.WithName("test.example.com/repo/tags/list")
	tagsList := []string{"tag1", "tag2", "funtag"}
	var m testutil.RequestResponseMap
	for i := 0; i < 3; i++ {
		body, err := json.Marshal(map[string]any{
			"name": "test.example.com/repo/tags/list",
			"tags": []string{tagsList[i]},
		})
		if err != nil {
			t.Fatal(err)
		}
		queryParams := make(map[string][]string)
		if i > 0 {
			queryParams["n"] = []string{"1"}
			queryParams["last"] = []string{tagsList[i-1]}
		}

		// Test both relative and absolute links.
		relativeLink := "/v2/" + repo.Name() + "/tags/list?n=1&last=" + tagsList[i]
		var link string
		switch i {
		case 0:
			link = relativeLink
		case len(tagsList) - 1:
			link = ""
		default:
			link = s.URL + relativeLink
		}

		headers := http.Header{
			"Content-Length": {fmt.Sprint(len(body))},
			"Last-Modified":  {time.Now().Add(-1 * time.Second).Format(time.ANSIC)},
		}
		if link != "" {
			headers.Set("Link", fmt.Sprintf(`<%s>; rel="next"`, link))
		}

		m = append(m, testutil.RequestResponseMapping{
			Request: testutil.Request{
				Method:      http.MethodGet,
				Route:       "/v2/" + repo.Name() + "/tags/list",
				QueryParams: queryParams,
			},
			Response: testutil.Response{
				StatusCode: http.StatusOK,
				Body:       body,
				Headers:    headers,
			},
		})
	}

	s.Config.Handler = testutil.NewHandler(m)

	r, err := NewRepository(repo, s.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	tagService := r.Tags(ctx)

	allTags, err := tagService.All(ctx)
	if err != nil {
		t.Fatal(allTags, err)
	}
	if len(allTags) != 3 {
		t.Fatalf("Wrong number of tags returned: %d, expected 3", len(allTags))
	}

	expected := map[string]struct{}{
		"tag1":   {},
		"tag2":   {},
		"funtag": {},
	}
	for _, t := range allTags {
		delete(expected, t)
	}
	if len(expected) != 0 {
		t.Fatalf("unexpected tags returned: %v", expected)
	}
}

func TestSanitizeLocation(t *testing.T) {
	for _, testcase := range []struct {
		description string
		location    string
		source      string
		expected    string
		err         error
	}{
		{
			description: "ensure relative location correctly resolved",
			location:    "/v2/foo/baasdf",
			source:      "http://blahalaja.com/v1",
			expected:    "http://blahalaja.com/v2/foo/baasdf",
		},
		{
			description: "ensure parameters are preserved",
			location:    "/v2/foo/baasdf?_state=asdfasfdasdfasdf&digest=foo",
			source:      "http://blahalaja.com/v1",
			expected:    "http://blahalaja.com/v2/foo/baasdf?_state=asdfasfdasdfasdf&digest=foo",
		},
		{
			description: "ensure new hostname overridden",
			location:    "https://mwhahaha.com/v2/foo/baasdf?_state=asdfasfdasdfasdf",
			source:      "http://blahalaja.com/v1",
			expected:    "https://mwhahaha.com/v2/foo/baasdf?_state=asdfasfdasdfasdf",
		},
	} {
		fatalf := func(format string, args ...any) {
			t.Fatalf(testcase.description+": "+format, args...)
		}

		s, err := sanitizeLocation(testcase.location, testcase.source)
		if !errors.Is(err, testcase.err) {
			if testcase.err != nil {
				fatalf("expected error: %v != %v", err, testcase)
			} else {
				fatalf("unexpected error sanitizing: %v", err)
			}
		}

		if s != testcase.expected {
			fatalf("bad sanitize: %q != %q", s, testcase.expected)
		}
	}
}
