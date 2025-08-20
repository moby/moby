package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/registry"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageCreateError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ImageCreate(context.Background(), "reference", ImageCreateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestImageCreate(t *testing.T) {
	const (
		expectedURL          = "/images/create"
		expectedImage        = "docker.io/test/my_image"
		expectedTag          = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		specifiedReference   = "test/my_image:latest@" + expectedTag
		expectedRegistryAuth = "eyJodHRwczovL2luZGV4LmRvY2tlci5pby92MS8iOnsiYXV0aCI6ImRHOTBid289IiwiZW1haWwiOiJqb2huQGRvZS5jb20ifX0="
	)

	client := &Client{
		client: newMockClient(func(r *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(r.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, r.URL)
			}
			registryAuth := r.Header.Get(registry.AuthHeader)
			if registryAuth != expectedRegistryAuth {
				return nil, fmt.Errorf("%s header not properly set in the request. Expected '%s', got %s", registry.AuthHeader, expectedRegistryAuth, registryAuth)
			}

			query := r.URL.Query()
			fromImage := query.Get("fromImage")
			if fromImage != expectedImage {
				return nil, fmt.Errorf("fromImage not set in URL query properly. Expected '%s', got %s", expectedImage, fromImage)
			}

			tag := query.Get("tag")
			if tag != expectedTag {
				return nil, fmt.Errorf("tag not set in URL query properly. Expected '%s', got %s", expectedTag, tag)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("body"))),
			}, nil
		}),
	}

	createResponse, err := client.ImageCreate(context.Background(), specifiedReference, ImageCreateOptions{
		RegistryAuth: expectedRegistryAuth,
	})
	assert.NilError(t, err)
	response, err := io.ReadAll(createResponse)
	assert.NilError(t, err)
	err = createResponse.Close()
	assert.NilError(t, err)
	assert.Check(t, is.Equal(string(response), "body"))
}
