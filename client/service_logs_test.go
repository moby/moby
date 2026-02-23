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
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.ServiceLogs(t.Context(), "service_id", ServiceLogsOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ServiceLogs(t.Context(), "service_id", ServiceLogsOptions{
		Since: "2006-01-02TZ",
	})
	assert.Check(t, is.ErrorContains(err, `parsing time "2006-01-02TZ"`))

	_, err = client.ServiceLogs(t.Context(), "", ServiceLogsOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ServiceLogs(t.Context(), "    ", ServiceLogsOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestServiceLogs(t *testing.T) {
	const expectedURL = "/services/service_id/logs"
	cases := []struct {
		doc                 string
		options             ServiceLogsOptions
		expectedQueryParams map[string]string
		expectedError       string
	}{
		{
			doc: "no options",
		},
		{
			// magic "all" value that's used as default in docker/cli
			// is equivalent to "don't send a tail option".
			doc: "tail all",
			options: ServiceLogsOptions{
				Tail: "all",
			},
		},
		{
			// TODO(thaJeztah): tail=0 currently means: "return zero lines"; perhaps we should just make it "all"
			//   or error, similar to "service logs"; https://github.com/moby/moby/blob/4d20b6fe56dfb2b06f4a5dd1f32913215a9c317b/daemon/cluster/services.go#L425-L449
			doc: "tail 0",
			options: ServiceLogsOptions{
				Tail: "0",
			},
			expectedQueryParams: map[string]string{
				"tail": "0",
			},
		},
		{
			doc: "tail",
			options: ServiceLogsOptions{
				Tail: "any",
			},
			expectedQueryParams: map[string]string{
				"tail": "any",
			},
		},
		{
			doc: "all options",
			options: ServiceLogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Timestamps: true,
				Details:    true,
				Follow:     true,
			},
			expectedQueryParams: map[string]string{
				"stdout":     "1",
				"stderr":     "1",
				"timestamps": "1",
				"details":    "1",
				"follow":     "1",
			},
		},
		{
			doc: "since",
			options: ServiceLogsOptions{
				// timestamp is passed as-is
				Since: "1136073600.000000001",
			},
			expectedQueryParams: map[string]string{
				"since": "1136073600.000000001",
			},
		},
		{
			doc: "invalid since",
			options: ServiceLogsOptions{
				// invalid dates are not passed.
				Since: "invalid value",
			},
			expectedError: `invalid value for "since": failed to parse value as time or duration: "invalid value"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.doc, func(t *testing.T) {
			client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
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
			}))
			assert.NilError(t, err)
			res, err := client.ServiceLogs(t.Context(), "service_id", tc.options)
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

func ExampleClient_ServiceLogs_withTimeout() {
	client, err := New(FromEnv)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := client.ServiceLogs(ctx, "service_id", ServiceLogsOptions{})
	if err != nil {
		log.Fatal(err)
	}
	defer res.Close()

	_, err = io.Copy(os.Stdout, res)
	if err != nil && !errors.Is(err, io.EOF) {
		log.Fatal(err)
	}
}
