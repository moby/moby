package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/system"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDiskUsageError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.DiskUsage(context.Background(), DiskUsageOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestDiskUsage(t *testing.T) {
	const expectedURL = "/system/df"
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
		}

		du := system.DiskUsage{
			LayersSize: int64(100),
			Images:     nil,
			Containers: nil,
			Volumes:    nil,
		}

		b, err := json.Marshal(du)
		if err != nil {
			return nil, err
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(b)),
		}, nil
	}))
	assert.NilError(t, err)
	_, err = client.DiskUsage(context.Background(), DiskUsageOptions{})
	assert.NilError(t, err)
}
