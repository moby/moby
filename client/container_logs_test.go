package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerLogsNotFoundError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusNotFound, "Not found")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerLogs(t.Context(), "container_id", ContainerLogsOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))

	_, err = client.ContainerLogs(t.Context(), "", ContainerLogsOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerLogs(t.Context(), "    ", ContainerLogsOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerLogsError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerLogs(t.Context(), "container_id", ContainerLogsOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerLogs(t.Context(), "container_id", ContainerLogsOptions{
		Since: "2006-01-02TZ",
	})
	assert.Check(t, is.ErrorContains(err, `parsing time "2006-01-02TZ"`))
	_, err = client.ContainerLogs(t.Context(), "container_id", ContainerLogsOptions{
		Until: "2006-01-02TZ",
	})
	assert.Check(t, is.ErrorContains(err, `parsing time "2006-01-02TZ"`))
}

func TestContainerLogs(t *testing.T) {
	const expectedURL = "/containers/container_id/logs"
	cases := []struct {
		doc                 string
		options             ContainerLogsOptions
		expectedQueryParams map[string]string
		expectedError       string
	}{
		{
			doc: "no options",
			expectedQueryParams: map[string]string{
				"tail": "",
			},
		},
		{
			doc: "tail",
			options: ContainerLogsOptions{
				Tail: "any",
			},
			expectedQueryParams: map[string]string{
				"tail": "any",
			},
		},
		{
			doc: "all options",
			options: ContainerLogsOptions{
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
			doc: "since",
			options: ContainerLogsOptions{
				// timestamp is passed as-is
				Since: "1136073600.000000001",
			},
			expectedQueryParams: map[string]string{
				"tail":  "",
				"since": "1136073600.000000001",
			},
		},
		{
			doc: "until",
			options: ContainerLogsOptions{
				// timestamp is passed as-is
				Until: "1136073600.000000001",
			},
			expectedQueryParams: map[string]string{
				"tail":  "",
				"until": "1136073600.000000001",
			},
		},
		{
			doc: "invalid since",
			options: ContainerLogsOptions{
				// invalid dates are not passed.
				Since: "invalid value",
			},
			expectedError: `invalid value for "since": failed to parse value as time or duration: "invalid value"`,
		},
		{
			doc: "invalid until",
			options: ContainerLogsOptions{
				// invalid dates are not passed.
				Until: "invalid value",
			},
			expectedError: `invalid value for "until": failed to parse value as time or duration: "invalid value"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.doc, func(t *testing.T) {
			client, err := New(
				WithMockClient(func(req *http.Request) (*http.Response, error) {
					if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
						return nil, err
					}
					// Check query parameters
					query := req.URL.Query()
					for key, expected := range tc.expectedQueryParams {
						actual := query.Get(key)
						if actual != expected {
							return nil, fmt.Errorf("%s not set in URL query properly. Expected '%s', got %s", key, expected, actual)
						}
					}
					return mockResponse(http.StatusOK, nil, "response")(req)
				}),
			)
			assert.NilError(t, err)
			res, err := client.ContainerLogs(t.Context(), "container_id", tc.options)
			if tc.expectedError != "" {
				assert.Check(t, is.Error(err, tc.expectedError))
				return
			}
			assert.NilError(t, err)
			defer func() { _ = res.Close() }()
			content, err := io.ReadAll(res)
			assert.NilError(t, err)
			assert.Check(t, is.Contains(string(content), "response"))
		})
	}
}

func ExampleClient_ContainerLogs_withTimeout() {
	client, err := New(FromEnv, WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := client.ContainerLogs(ctx, "container_id", ContainerLogsOptions{})
	if err != nil {
		log.Fatal(err)
	}
	defer res.Close()

	_, err = io.Copy(os.Stdout, res)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Fatal(err)
	}
}
