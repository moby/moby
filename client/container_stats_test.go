package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client/containerstats"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerStatsError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.ContainerStats(context.Background(), "nothing")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerStats(context.Background(), "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerStats(context.Background(), "    ")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerStats(t *testing.T) {
	const expectedURL = "/containers/container_id/stats"
	tests := []struct {
		stream         bool
		expectedStream string
	}{
		{
			expectedStream: "0",
		},
		{
			stream:         true,
			expectedStream: "1",
		},
	}
	for _, tc := range tests {
		client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
				return nil, err
			}

			query := req.URL.Query()
			stream := query.Get("stream")
			if stream != tc.expectedStream {
				return nil, fmt.Errorf("stream not set in URL query properly. Expected '%s', got %s", tc.expectedStream, stream)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("{}"))),
			}, nil
		}))
		assert.NilError(t, err)

		var opts []containerstats.Option
		stream := make(chan containerstats.StreamItem)
		oneshot := container.StatsResponse{}
		if tc.stream {
			opts = append(opts, containerstats.WithStream(stream))
		} else {
			opts = append(opts, containerstats.WithOneshot(&oneshot))
		}

		_, err = client.ContainerStats(context.Background(), "container_id",
			opts...,
		)
		assert.NilError(t, err)
	}
}
