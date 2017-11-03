package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
)

func TestContainerLogsNotFoundError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "Not found")),
	}
	_, err := client.ContainerLogs(context.Background(), "container_id", types.ContainerLogsOptions{})
	if !IsErrNotFound(err) {
		t.Fatalf("expected a not found error, got %v", err)
	}
}

func TestContainerLogsError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ContainerLogs(context.Background(), "container_id", types.ContainerLogsOptions{})
	assert.Check(t, is.Error(err, "Error response from daemon: Server error"))
	_, err = client.ContainerLogs(context.Background(), "container_id", types.ContainerLogsOptions{
		Since: "2006-01-02TZ",
	})
	assert.Check(t, is.ErrorContains(err, `parsing time "2006-01-02TZ"`))
	_, err = client.ContainerLogs(context.Background(), "container_id", types.ContainerLogsOptions{
		Until: "2006-01-02TZ",
	})
	assert.Check(t, is.ErrorContains(err, `parsing time "2006-01-02TZ"`))
}

func TestContainerLogs(t *testing.T) {
	expectedURL := "/containers/container_id/logs"
	cases := []struct {
		options             types.ContainerLogsOptions
		expectedQueryParams map[string]string
		expectedError       string
	}{
		{
			expectedQueryParams: map[string]string{
				"tail": "",
			},
		},
		{
			options: types.ContainerLogsOptions{
				Tail: "any",
			},
			expectedQueryParams: map[string]string{
				"tail": "any",
			},
		},
		{
			options: types.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Timestamps: true,
				Details:    true,
				Follow:     true,
			},
			expectedQueryParams: map[string]string{
				"tail":       "",
				"stdout":     "1",
				"stderr":     "1",
				"timestamps": "1",
				"details":    "1",
				"follow":     "1",
			},
		},
		{
			options: types.ContainerLogsOptions{
				// timestamp will be passed as is
				Since: "1136073600.000000001",
			},
			expectedQueryParams: map[string]string{
				"tail":  "",
				"since": "1136073600.000000001",
			},
		},
		{
			options: types.ContainerLogsOptions{
				// timestamp will be passed as is
				Until: "1136073600.000000001",
			},
			expectedQueryParams: map[string]string{
				"tail":  "",
				"until": "1136073600.000000001",
			},
		},
		{
			options: types.ContainerLogsOptions{
				// An complete invalid date will not be passed
				Since: "invalid value",
			},
			expectedError: `invalid value for "since": failed to parse value as time or duration: "invalid value"`,
		},
		{
			options: types.ContainerLogsOptions{
				// An complete invalid date will not be passed
				Until: "invalid value",
			},
			expectedError: `invalid value for "until": failed to parse value as time or duration: "invalid value"`,
		},
	}
	for _, logCase := range cases {
		client := &Client{
			client: newMockClient(func(r *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(r.URL.Path, expectedURL) {
					return nil, fmt.Errorf("expected URL '%s', got '%s'", expectedURL, r.URL)
				}
				// Check query parameters
				query := r.URL.Query()
				for key, expected := range logCase.expectedQueryParams {
					actual := query.Get(key)
					if actual != expected {
						return nil, fmt.Errorf("%s not set in URL query properly. Expected '%s', got %s", key, expected, actual)
					}
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewReader([]byte("response"))),
				}, nil
			}),
		}
		body, err := client.ContainerLogs(context.Background(), "container_id", logCase.options)
		if logCase.expectedError != "" {
			assert.Check(t, is.Error(err, logCase.expectedError))
			continue
		}
		assert.NilError(t, err)
		defer body.Close()
		content, err := ioutil.ReadAll(body)
		assert.NilError(t, err)
		assert.Check(t, is.Contains(string(content), "response"))
	}
}

func ExampleClient_ContainerLogs_withTimeout() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, _ := NewEnvClient()
	reader, err := client.ContainerLogs(ctx, "container_id", types.ContainerLogsOptions{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = io.Copy(os.Stdout, reader)
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}
}
