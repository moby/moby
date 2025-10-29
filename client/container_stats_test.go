package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerStatsError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.ContainerStats(t.Context(), "nothing", ContainerStatsOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerStats(t.Context(), "", ContainerStatsOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerStats(t.Context(), "    ", ContainerStatsOptions{})
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
			expectedStream: "false",
		},
		{
			stream:         true,
			expectedStream: "true",
		},
	}
	for _, tc := range tests {
		client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
				return nil, err
			}

			query := req.URL.Query()
			stream := query.Get("stream")
			if stream != tc.expectedStream {
				return nil, fmt.Errorf("stream not set in URL query properly. Expected '%s', got %s", tc.expectedStream, stream)
			}
			return mockJSONResponse(http.StatusOK, nil, container.StatsResponse{ID: "container_id"})(req)
		}))
		assert.NilError(t, err)
		resp, err := client.ContainerStats(t.Context(), "container_id", ContainerStatsOptions{
			Stream: tc.stream,
		})
		assert.NilError(t, err)
		t.Cleanup(func() {
			_ = resp.Body.Close()
		})
		content, err := io.ReadAll(resp.Body)
		assert.NilError(t, err)

		var stats container.StatsResponse
		err = json.Unmarshal(content, &stats)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(stats.ID, "container_id"))
	}
}
