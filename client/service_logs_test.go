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

func TestServiceLogsError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.ServiceLogs(context.Background(), "service_id", ServiceLogsOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ServiceLogs(context.Background(), "service_id", ServiceLogsOptions{
		Since: "2006-01-02TZ",
	})
	assert.Check(t, is.ErrorContains(err, `parsing time "2006-01-02TZ"`))

	_, err = client.ServiceLogs(context.Background(), "", ServiceLogsOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ServiceLogs(context.Background(), "    ", ServiceLogsOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestServiceLogs(t *testing.T) {
	const expectedURL = "/services/service_id/logs"
	cases := []struct {
		options             ServiceLogsOptions
		expectedQueryParams map[string]string
		expectedError       string
	}{
		{
			expectedQueryParams: map[string]string{
				"tail": "",
			},
		},
		{
			options: ServiceLogsOptions{
				Tail: "any",
			},
			expectedQueryParams: map[string]string{
				"tail": "any",
			},
		},
		{
			options: ServiceLogsOptions{
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
			options: ServiceLogsOptions{
				// timestamp is passed as-is
				Since: "1136073600.000000001",
			},
			expectedQueryParams: map[string]string{
				"tail":  "",
				"since": "1136073600.000000001",
			},
		},
		{
			options: ServiceLogsOptions{
				// invalid dates are not passed.
				Since: "invalid value",
			},
			expectedError: `invalid value for "since": failed to parse value as time or duration: "invalid value"`,
		},
	}
	for _, logCase := range cases {
		client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
				return nil, err
			}
			// Check query parameters
			query := req.URL.Query()
			for key, expected := range logCase.expectedQueryParams {
				actual := query.Get(key)
				if actual != expected {
					return nil, fmt.Errorf("%s not set in URL query properly. Expected '%s', got %s", key, expected, actual)
				}
			}
			return mockResponse(http.StatusOK, nil, "response")(req)
		}))
		assert.NilError(t, err)
		body, err := client.ServiceLogs(context.Background(), "service_id", logCase.options)
		if logCase.expectedError != "" {
			assert.Check(t, is.Error(err, logCase.expectedError))
			continue
		}
		assert.NilError(t, err)
		defer body.Close()
		content, err := io.ReadAll(body)
		assert.NilError(t, err)
		assert.Check(t, is.Contains(string(content), "response"))
	}
}

func ExampleClient_ServiceLogs_withTimeout() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, _ := NewClientWithOpts(FromEnv)
	reader, err := client.ServiceLogs(ctx, "service_id", ServiceLogsOptions{})
	if err != nil {
		log.Fatal(err)
	}

	_, err = io.Copy(os.Stdout, reader)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Fatal(err)
	}
}
