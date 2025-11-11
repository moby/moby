package registryclient

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/docker/distribution"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/moby/moby/v2/daemon/internal/registryclient/testutil"
)

// Test implements distribution.BlobWriter
var _ distribution.BlobWriter = &httpBlobUpload{}

func TestUploadReadFrom(t *testing.T) {
	_, b := newRandomBlob(64)
	repo := "test/upload/readfrom"
	locationPath := fmt.Sprintf("/v2/%s/uploads/testid", repo)

	m := testutil.RequestResponseMap([]testutil.RequestResponseMapping{
		{
			Request: testutil.Request{
				Method: http.MethodGet,
				Route:  "/v2/",
			},
			Response: testutil.Response{
				StatusCode: http.StatusOK,
				Headers: http.Header{
					"Docker-Distribution-API-Version": {"registry/2.0"},
				},
			},
		},
		// Test Valid case
		{
			Request: testutil.Request{
				Method: http.MethodPatch,
				Route:  locationPath,
				Body:   b,
			},
			Response: testutil.Response{
				StatusCode: http.StatusAccepted,
				Headers: http.Header{
					"Docker-Upload-UUID": {"46603072-7a1b-4b41-98f9-fd8a7da89f9b"},
					"Location":           {locationPath},
					"Range":              {"0-63"},
				},
			},
		},
		// Test invalid range
		{
			Request: testutil.Request{
				Method: http.MethodPatch,
				Route:  locationPath,
				Body:   b,
			},
			Response: testutil.Response{
				StatusCode: http.StatusAccepted,
				Headers: http.Header(map[string][]string{
					"Docker-Upload-UUID": {"46603072-7a1b-4b41-98f9-fd8a7da89f9b"},
					"Location":           {locationPath},
					"Range":              {""},
				}),
			},
		},
		// Test 404
		{
			Request: testutil.Request{
				Method: http.MethodPatch,
				Route:  locationPath,
				Body:   b,
			},
			Response: testutil.Response{
				StatusCode: http.StatusNotFound,
			},
		},
		// Test 400 valid json
		{
			Request: testutil.Request{
				Method: http.MethodPatch,
				Route:  locationPath,
				Body:   b,
			},
			Response: testutil.Response{
				StatusCode: http.StatusBadRequest,
				Headers:    http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
				Body: []byte(`
					{ "errors":
						[
							{
								"code": "BLOB_UPLOAD_INVALID",
								"message": "blob upload invalid",
								"detail": "more detail"
							}
						]
					} `),
			},
		},
		// Test 400 invalid json
		{
			Request: testutil.Request{
				Method: http.MethodPatch,
				Route:  locationPath,
				Body:   b,
			},
			Response: testutil.Response{
				StatusCode: http.StatusBadRequest,
				Headers:    http.Header{"Content-Type": []string{"application/json"}},
				Body:       []byte("something bad happened"),
			},
		},
		// Test 500
		{
			Request: testutil.Request{
				Method: http.MethodPatch,
				Route:  locationPath,
				Body:   b,
			},
			Response: testutil.Response{
				StatusCode: http.StatusInternalServerError,
			},
		},
	})

	e, c := testServer(m)
	defer c()

	blobUpload := &httpBlobUpload{
		client: &http.Client{},
	}

	// Valid case
	blobUpload.location = e + locationPath
	n, err := blobUpload.ReadFrom(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("Error calling ReadFrom: %s", err)
	}
	if n != 64 {
		t.Fatalf("Wrong length returned from ReadFrom: %d, expected 64", n)
	}

	// Bad range
	blobUpload.location = e + locationPath
	_, err = blobUpload.ReadFrom(bytes.NewReader(b))
	if err == nil {
		t.Fatalf("Expected error when bad range received")
	}

	// 404
	blobUpload.location = e + locationPath
	_, err = blobUpload.ReadFrom(bytes.NewReader(b))
	if err == nil {
		t.Fatalf("Expected error when not found")
	}
	if !errors.Is(err, distribution.ErrBlobUploadUnknown) {
		t.Fatalf("Wrong error thrown: %s, expected %s", err, distribution.ErrBlobUploadUnknown)
	}

	// 400 valid json
	blobUpload.location = e + locationPath
	_, err = blobUpload.ReadFrom(bytes.NewReader(b))
	if err == nil {
		t.Fatalf("Expected error when not found")
	}
	var uploadErr errcode.Errors
	if !errors.As(err, &uploadErr) {
		t.Fatalf("Wrong error type %T: %s", err, err)
	}

	// 400 invalid json
	blobUpload.location = e + locationPath
	_, err = blobUpload.ReadFrom(bytes.NewReader(b))
	if err == nil {
		t.Fatalf("Expected error when not found")
	}
	if uploadErr, ok := err.(*UnexpectedHTTPResponseError); !ok {
		t.Fatalf("Wrong error type %T: %s", err, err)
	} else {
		respStr := string(uploadErr.Response)
		if expected := "something bad happened"; respStr != expected {
			t.Fatalf("Unexpected response string: %s, expected: %s", respStr, expected)
		}
	}

	// 500
	blobUpload.location = e + locationPath
	_, err = blobUpload.ReadFrom(bytes.NewReader(b))
	if err == nil {
		t.Fatalf("Expected error when not found")
	}
	if uploadErr, ok := err.(*UnexpectedHTTPStatusError); !ok {
		t.Fatalf("Wrong error type %T: %s", err, err)
	} else if expected := "500 " + http.StatusText(http.StatusInternalServerError); uploadErr.Status != expected {
		t.Fatalf("Unexpected response status: %s, expected %s", uploadErr.Status, expected)
	}
}
