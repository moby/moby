package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerStatsError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.ContainerStats(context.Background(), "nothing", false)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerStats(context.Background(), "", false)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerStats(context.Background(), "    ", false)
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
			return mockResponse(http.StatusOK, nil, "response")(req)
		}))
		assert.NilError(t, err)
		resp, err := client.ContainerStats(context.Background(), "container_id", tc.stream)
		assert.NilError(t, err)
		t.Cleanup(func() {
			_ = resp.Body.Close()
		})
		content, err := io.ReadAll(resp.Body)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(string(content), "response"))
	}
}
