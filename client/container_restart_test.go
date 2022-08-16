package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
)

func TestContainerRestartError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	err := client.ContainerRestart(context.Background(), "nothing", container.StopOptions{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestContainerRestart(t *testing.T) {
	const expectedURL = "/v1.42/containers/container_id/restart"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			s := req.URL.Query().Get("signal")
			if s != "SIGKILL" {
				return nil, fmt.Errorf("signal not set in URL query. Expected 'SIGKILL', got '%s'", s)
			}
			t := req.URL.Query().Get("t")
			if t != "100" {
				return nil, fmt.Errorf("t (timeout) not set in URL query properly. Expected '100', got %s", t)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
		version: "1.42",
	}
	timeout := 100
	err := client.ContainerRestart(context.Background(), "container_id", container.StopOptions{
		Signal:  "SIGKILL",
		Timeout: &timeout,
	})
	if err != nil {
		t.Fatal(err)
	}
}
