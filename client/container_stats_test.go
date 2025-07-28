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
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerStatsError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ContainerStats(context.Background(), "nothing", false)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerStats(context.Background(), "", false)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerStats(context.Background(), "    ", false)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerStats(t *testing.T) {
	expectedURL := "/containers/container_id/stats"
	cases := []struct {
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
	for _, c := range cases {
		client := &Client{
			client: newMockClient(func(r *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(r.URL.Path, expectedURL) {
					return nil, fmt.Errorf("expected URL '%s', got '%s'", expectedURL, r.URL)
				}

				query := r.URL.Query()
				stream := query.Get("stream")
				if stream != c.expectedStream {
					return nil, fmt.Errorf("stream not set in URL query properly. Expected '%s', got %s", c.expectedStream, stream)
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte("response"))),
				}, nil
			}),
		}
		resp, err := client.ContainerStats(context.Background(), "container_id", c.stream)
		assert.NilError(t, err)
		t.Cleanup(func() {
			_ = resp.Body.Close()
		})
		content, err := io.ReadAll(resp.Body)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(string(content), "response"))
	}
}
