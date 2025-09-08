package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageInspectError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.ImageInspect(context.Background(), "nothing")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestImageInspectImageNotFound(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusNotFound, "Server error")))
	assert.NilError(t, err)

	_, err = client.ImageInspect(context.Background(), "unknown")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestImageInspectWithEmptyID(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("should not make request")
	}))
	assert.NilError(t, err)
	_, err = client.ImageInspect(context.Background(), "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestImageInspect(t *testing.T) {
	expectedURL := "/images/image_id/json"
	expectedTags := []string{"tag1", "tag2"}
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if !strings.HasPrefix(req.URL.Path, expectedURL) {
			return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
		}
		content, err := json.Marshal(image.InspectResponse{
			ID:       "image_id",
			RepoTags: expectedTags,
		})
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(content)),
		}, nil
	}))
	assert.NilError(t, err)

	imageInspect, err := client.ImageInspect(context.Background(), "image_id")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(imageInspect.ID, "image_id"))
	assert.Check(t, is.DeepEqual(imageInspect.RepoTags, expectedTags))
}

func TestImageInspectWithPlatform(t *testing.T) {
	expectedURL := "/images/image_id/json"
	requestedPlatform := &ocispec.Platform{
		OS:           "linux",
		Architecture: "arm64",
	}

	expectedPlatform, err := encodePlatform(requestedPlatform)
	assert.NilError(t, err)

	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if !strings.HasPrefix(req.URL.Path, expectedURL) {
			return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
		}

		// Check if platform parameter is passed correctly
		platform := req.URL.Query().Get("platform")
		if platform != expectedPlatform {
			return nil, fmt.Errorf("Expected platform '%s', got '%s'", expectedPlatform, platform)
		}

		content, err := json.Marshal(image.InspectResponse{
			ID:           "image_id",
			Architecture: "arm64",
			Os:           "linux",
		})
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(content)),
		}, nil
	}))
	assert.NilError(t, err)

	var opts []ImageInspectOption
	if requestedPlatform != nil {
		opts = append(opts, WithPlatform(*requestedPlatform))
	}

	imageInspect, err := client.ImageInspect(context.Background(), "image_id", opts...)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(imageInspect.ID, "image_id"))
	assert.Check(t, is.Equal(imageInspect.Architecture, "arm64"))
	assert.Check(t, is.Equal(imageInspect.Os, "linux"))
}
