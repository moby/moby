package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	"github.com/docker/distribution/testutil"
)

type testBlob struct {
	digest   digest.Digest
	contents []byte
}

func TestRangeHeaderParser(t *testing.T) {
	const (
		malformedRangeHeader = "bytes=0-A/C"
		emptyRangeHeader     = ""
		rFirst               = 100
		rSecond              = 200
	)

	var (
		wellformedRangeHeader = fmt.Sprintf("bytes=0-%d/%d", rFirst, rSecond)
	)

	if _, _, err := parseRangeHeader(malformedRangeHeader); err == nil {
		t.Fatalf("malformedRangeHeader: error expected, got nil")
	}

	if _, _, err := parseRangeHeader(emptyRangeHeader); err == nil {
		t.Fatalf("emptyRangeHeader: error expected, got nil")
	}

	first, second, err := parseRangeHeader(wellformedRangeHeader)
	if err != nil {
		t.Fatalf("wellformedRangeHeader: unexpected error %v", err)
	}

	if first != rFirst || second != rSecond {
		t.Fatalf("Range has been parsed unproperly: %d/%d", first, second)
	}

}

func TestPush(t *testing.T) {
	name := "hello/world"
	tag := "sometag"
	testBlobs := []testBlob{
		{
			digest:   "tarsum.v2+sha256:12345",
			contents: []byte("some contents"),
		},
		{
			digest:   "tarsum.v2+sha256:98765",
			contents: []byte("some other contents"),
		},
	}
	uploadLocations := make([]string, len(testBlobs))
	blobs := make([]manifest.FSLayer, len(testBlobs))
	history := make([]manifest.History, len(testBlobs))

	for i, blob := range testBlobs {
		// TODO(bbland): this is returning the same location for all uploads,
		// because we can't know which blob will get which location.
		// It's sort of okay because we're using unique digests, but this needs
		// to change at some point.
		uploadLocations[i] = fmt.Sprintf("/v2/%s/blobs/test-uuid", name)
		blobs[i] = manifest.FSLayer{BlobSum: blob.digest}
		history[i] = manifest.History{V1Compatibility: blob.digest.String()}
	}

	m := &manifest.SignedManifest{
		Manifest: manifest.Manifest{
			Name:         name,
			Tag:          tag,
			Architecture: "x86",
			FSLayers:     blobs,
			History:      history,
			Versioned: manifest.Versioned{
				SchemaVersion: 1,
			},
		},
	}
	var err error
	m.Raw, err = json.Marshal(m)

	blobRequestResponseMappings := make([]testutil.RequestResponseMapping, 2*len(testBlobs))
	for i, blob := range testBlobs {
		blobRequestResponseMappings[2*i] = testutil.RequestResponseMapping{
			Request: testutil.Request{
				Method: "POST",
				Route:  "/v2/" + name + "/blobs/uploads/",
			},
			Response: testutil.Response{
				StatusCode: http.StatusAccepted,
				Headers: http.Header(map[string][]string{
					"Location": {uploadLocations[i]},
				}),
			},
		}
		blobRequestResponseMappings[2*i+1] = testutil.RequestResponseMapping{
			Request: testutil.Request{
				Method: "PUT",
				Route:  uploadLocations[i],
				QueryParams: map[string][]string{
					"digest": {blob.digest.String()},
				},
				Body: blob.contents,
			},
			Response: testutil.Response{
				StatusCode: http.StatusCreated,
			},
		}
	}

	handler := testutil.NewHandler(append(blobRequestResponseMappings, testutil.RequestResponseMapping{
		Request: testutil.Request{
			Method: "PUT",
			Route:  "/v2/" + name + "/manifests/" + tag,
			Body:   m.Raw,
		},
		Response: testutil.Response{
			StatusCode: http.StatusOK,
		},
	}))
	var server *httptest.Server

	// HACK(stevvooe): Super hack to follow: the request response map approach
	// above does not let us correctly format the location header to the
	// server url. This handler intercepts and re-writes the location header
	// to the server url.

	hack := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w = &headerInterceptingResponseWriter{ResponseWriter: w, serverURL: server.URL}
		handler.ServeHTTP(w, r)
	})

	server = httptest.NewServer(hack)
	client, err := New(server.URL)
	if err != nil {
		t.Fatalf("error creating client: %v", err)
	}
	objectStore := &memoryObjectStore{
		mutex:           new(sync.Mutex),
		manifestStorage: make(map[string]*manifest.SignedManifest),
		layerStorage:    make(map[digest.Digest]Layer),
	}

	for _, blob := range testBlobs {
		l, err := objectStore.Layer(blob.digest)
		if err != nil {
			t.Fatal(err)
		}

		writer, err := l.Writer()
		if err != nil {
			t.Fatal(err)
		}

		writer.SetSize(len(blob.contents))
		writer.Write(blob.contents)
		writer.Close()
	}

	objectStore.WriteManifest(name, tag, m)

	err = Push(client, objectStore, name, tag)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPull(t *testing.T) {
	name := "hello/world"
	tag := "sometag"
	testBlobs := []testBlob{
		{
			digest:   "tarsum.v2+sha256:12345",
			contents: []byte("some contents"),
		},
		{
			digest:   "tarsum.v2+sha256:98765",
			contents: []byte("some other contents"),
		},
	}
	blobs := make([]manifest.FSLayer, len(testBlobs))
	history := make([]manifest.History, len(testBlobs))

	for i, blob := range testBlobs {
		blobs[i] = manifest.FSLayer{BlobSum: blob.digest}
		history[i] = manifest.History{V1Compatibility: blob.digest.String()}
	}

	m := &manifest.SignedManifest{
		Manifest: manifest.Manifest{
			Name:         name,
			Tag:          tag,
			Architecture: "x86",
			FSLayers:     blobs,
			History:      history,
			Versioned: manifest.Versioned{
				SchemaVersion: 1,
			},
		},
	}
	manifestBytes, err := json.Marshal(m)

	blobRequestResponseMappings := make([]testutil.RequestResponseMapping, len(testBlobs))
	for i, blob := range testBlobs {
		blobRequestResponseMappings[i] = testutil.RequestResponseMapping{
			Request: testutil.Request{
				Method: "GET",
				Route:  "/v2/" + name + "/blobs/" + blob.digest.String(),
			},
			Response: testutil.Response{
				StatusCode: http.StatusOK,
				Body:       blob.contents,
			},
		}
	}

	handler := testutil.NewHandler(append(blobRequestResponseMappings, testutil.RequestResponseMapping{
		Request: testutil.Request{
			Method: "GET",
			Route:  "/v2/" + name + "/manifests/" + tag,
		},
		Response: testutil.Response{
			StatusCode: http.StatusOK,
			Body:       manifestBytes,
		},
	}))
	server := httptest.NewServer(handler)
	client, err := New(server.URL)
	if err != nil {
		t.Fatalf("error creating client: %v", err)
	}
	objectStore := &memoryObjectStore{
		mutex:           new(sync.Mutex),
		manifestStorage: make(map[string]*manifest.SignedManifest),
		layerStorage:    make(map[digest.Digest]Layer),
	}

	err = Pull(client, objectStore, name, tag)
	if err != nil {
		t.Fatal(err)
	}

	m, err = objectStore.Manifest(name, tag)
	if err != nil {
		t.Fatal(err)
	}

	mBytes, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	if string(mBytes) != string(manifestBytes) {
		t.Fatal("Incorrect manifest")
	}

	for _, blob := range testBlobs {
		l, err := objectStore.Layer(blob.digest)
		if err != nil {
			t.Fatal(err)
		}

		reader, err := l.Reader()
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()

		blobBytes, err := ioutil.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}

		if string(blobBytes) != string(blob.contents) {
			t.Fatal("Incorrect blob")
		}
	}
}

func TestPullResume(t *testing.T) {
	name := "hello/world"
	tag := "sometag"
	testBlobs := []testBlob{
		{
			digest:   "tarsum.v2+sha256:12345",
			contents: []byte("some contents"),
		},
		{
			digest:   "tarsum.v2+sha256:98765",
			contents: []byte("some other contents"),
		},
	}
	layers := make([]manifest.FSLayer, len(testBlobs))
	history := make([]manifest.History, len(testBlobs))

	for i, layer := range testBlobs {
		layers[i] = manifest.FSLayer{BlobSum: layer.digest}
		history[i] = manifest.History{V1Compatibility: layer.digest.String()}
	}

	m := &manifest.Manifest{
		Name:         name,
		Tag:          tag,
		Architecture: "x86",
		FSLayers:     layers,
		History:      history,
		Versioned: manifest.Versioned{
			SchemaVersion: 1,
		},
	}
	manifestBytes, err := json.Marshal(m)

	layerRequestResponseMappings := make([]testutil.RequestResponseMapping, 2*len(testBlobs))
	for i, blob := range testBlobs {
		layerRequestResponseMappings[2*i] = testutil.RequestResponseMapping{
			Request: testutil.Request{
				Method: "GET",
				Route:  "/v2/" + name + "/blobs/" + blob.digest.String(),
			},
			Response: testutil.Response{
				StatusCode: http.StatusOK,
				Body:       blob.contents[:len(blob.contents)/2],
				Headers: http.Header(map[string][]string{
					"Content-Length": {fmt.Sprint(len(blob.contents))},
				}),
			},
		}
		layerRequestResponseMappings[2*i+1] = testutil.RequestResponseMapping{
			Request: testutil.Request{
				Method: "GET",
				Route:  "/v2/" + name + "/blobs/" + blob.digest.String(),
			},
			Response: testutil.Response{
				StatusCode: http.StatusOK,
				Body:       blob.contents[len(blob.contents)/2:],
			},
		}
	}

	for i := 0; i < 3; i++ {
		layerRequestResponseMappings = append(layerRequestResponseMappings, testutil.RequestResponseMapping{
			Request: testutil.Request{
				Method: "GET",
				Route:  "/v2/" + name + "/manifests/" + tag,
			},
			Response: testutil.Response{
				StatusCode: http.StatusOK,
				Body:       manifestBytes,
			},
		})
	}

	handler := testutil.NewHandler(layerRequestResponseMappings)
	server := httptest.NewServer(handler)
	client, err := New(server.URL)
	if err != nil {
		t.Fatalf("error creating client: %v", err)
	}
	objectStore := &memoryObjectStore{
		mutex:           new(sync.Mutex),
		manifestStorage: make(map[string]*manifest.SignedManifest),
		layerStorage:    make(map[digest.Digest]Layer),
	}

	for attempts := 0; attempts < 3; attempts++ {
		err = Pull(client, objectStore, name, tag)
		if err == nil {
			break
		}
	}

	if err != nil {
		t.Fatal(err)
	}

	sm, err := objectStore.Manifest(name, tag)
	if err != nil {
		t.Fatal(err)
	}

	mBytes, err := json.Marshal(sm)
	if err != nil {
		t.Fatal(err)
	}

	if string(mBytes) != string(manifestBytes) {
		t.Fatal("Incorrect manifest")
	}

	for _, blob := range testBlobs {
		l, err := objectStore.Layer(blob.digest)
		if err != nil {
			t.Fatal(err)
		}

		reader, err := l.Reader()
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()

		layerBytes, err := ioutil.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}

		if string(layerBytes) != string(blob.contents) {
			t.Fatal("Incorrect blob")
		}
	}
}

// headerInterceptingResponseWriter is a hacky workaround to re-write the
// location header to have the server url.
type headerInterceptingResponseWriter struct {
	http.ResponseWriter
	serverURL string
}

func (hirw *headerInterceptingResponseWriter) WriteHeader(status int) {
	location := hirw.Header().Get("Location")
	if location != "" {
		hirw.Header().Set("Location", hirw.serverURL+location)
	}

	hirw.ResponseWriter.WriteHeader(status)
}
