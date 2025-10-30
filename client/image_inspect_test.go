package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageInspectError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.ImageInspect(context.Background(), "nothing")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestImageInspectImageNotFound(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusNotFound, "Server error")))
	assert.NilError(t, err)

	_, err = client.ImageInspect(context.Background(), "unknown")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestImageInspectWithEmptyID(t *testing.T) {
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("should not make request")
	}))
	assert.NilError(t, err)
	_, err = client.ImageInspect(context.Background(), "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestImageInspect(t *testing.T) {
	const expectedURL = "/images/image_id/json"
	expectedTags := []string{"tag1", "tag2"}
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
		}
		return mockJSONResponse(http.StatusOK, nil, image.InspectResponse{
			ID:       "image_id",
			RepoTags: expectedTags,
		})(req)
	}))
	assert.NilError(t, err)

	imageInspect, err := client.ImageInspect(context.Background(), "image_id")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(imageInspect.ID, "image_id"))
	assert.Check(t, is.DeepEqual(imageInspect.RepoTags, expectedTags))
}

func TestImageInspectWithPlatform(t *testing.T) {
	const expectedURL = "/images/image_id/json"
	requestedPlatform := &ocispec.Platform{
		OS:           "linux",
		Architecture: "arm64",
	}

	expectedPlatform, err := encodePlatform(requestedPlatform)
	assert.NilError(t, err)

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
		}

		// Check if platform parameter is passed correctly
		platform := req.URL.Query().Get("platform")
		if platform != expectedPlatform {
			return nil, fmt.Errorf("Expected platform '%s', got '%s'", expectedPlatform, platform)
		}

		return mockJSONResponse(http.StatusOK, nil, image.InspectResponse{
			ID:           "image_id",
			Architecture: "arm64",
			Os:           "linux",
		})(req)
	}))
	assert.NilError(t, err)

	imageInspect, err := client.ImageInspect(context.Background(), "image_id", ImageInspectWithPlatform(requestedPlatform))
	assert.NilError(t, err)
	assert.Check(t, is.Equal(imageInspect.ID, "image_id"))
	assert.Check(t, is.Equal(imageInspect.Architecture, "arm64"))
	assert.Check(t, is.Equal(imageInspect.Os, "linux"))
}
