package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestImageLoadError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	_, err = client.ImageLoad(context.Background(), nil, true)
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestImageLoad(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/images/load"
	expectedInput := "inputBody"
	expectedOutput := "outputBody"
	loadCases := []struct {
		quiet                bool
		responseContentType  string
		expectedResponseJSON bool
		expectedQueryParams  map[string]string
	}{
		{
			quiet:                false,
			responseContentType:  "text/plain",
			expectedResponseJSON: false,
			expectedQueryParams: map[string]string{
				"quiet": "0",
			},
		},
		{
			quiet:                true,
			responseContentType:  "application/json",
			expectedResponseJSON: true,
			expectedQueryParams: map[string]string{
				"quiet": "1",
			},
		},
	}
	for _, loadCase := range loadCases {
		client, err := NewClientWithOpts(
			WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, expectedURL) {
					return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
				}
				contentType := req.Header.Get("Content-Type")
				if contentType != "application/x-tar" {
					return nil, fmt.Errorf("content-type not set in URL headers properly. Expected 'application/x-tar', got %s", contentType)
				}
				query := req.URL.Query()
				for key, expected := range loadCase.expectedQueryParams {
					actual := query.Get(key)
					if actual != expected {
						return nil, fmt.Errorf("%s not set in URL query properly. Expected '%s', got %s", key, expected, actual)
					}
				}
				headers := http.Header{}
				headers.Add("Content-Type", loadCase.responseContentType)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte(expectedOutput))),
					Header:     headers,
				}, nil
			})),
		)
		assert.NilError(t, err)

		input := bytes.NewReader([]byte(expectedInput))
		imageLoadResponse, err := client.ImageLoad(context.Background(), input, loadCase.quiet)
		if err != nil {
			t.Fatal(err)
		}
		if imageLoadResponse.JSON != loadCase.expectedResponseJSON {
			t.Fatalf("expected a JSON response, was not.")
		}
		body, err := io.ReadAll(imageLoadResponse.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != expectedOutput {
			t.Fatalf("expected %s, got %s", expectedOutput, string(body))
		}
	}
}
