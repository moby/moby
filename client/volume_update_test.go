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
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVolumeUpdateError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	err = client.VolumeUpdate(context.Background(), "volume", swarm.Version{}, VolumeUpdateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.VolumeUpdate(context.Background(), "", swarm.Version{}, VolumeUpdateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.VolumeUpdate(context.Background(), "    ", swarm.Version{}, VolumeUpdateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestVolumeUpdate(t *testing.T) {
	const (
		expectedURL     = "/volumes/test1"
		expectedVersion = "version=10"
	)

	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPut, expectedURL); err != nil {
			return nil, err
		}
		if !strings.Contains(req.URL.RawQuery, expectedVersion) {
			return nil, fmt.Errorf("expected query to contain '%s', got '%s'", expectedVersion, req.URL.RawQuery)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte("body"))),
		}, nil
	}))
	assert.NilError(t, err)

	err = client.VolumeUpdate(context.Background(), "test1", swarm.Version{Index: uint64(10)}, VolumeUpdateOptions{})
	assert.NilError(t, err)
}
