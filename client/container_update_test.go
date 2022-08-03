package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestContainerUpdateError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)
	_, err = client.ContainerUpdate(context.Background(), "nothing", container.UpdateConfig{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestContainerUpdate(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/containers/container_id/update"

	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}

			b, err := json.Marshal(container.ContainerUpdateOKBody{})
			if err != nil {
				return nil, err
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}, nil
		})),
	)
	assert.NilError(t, err)

	_, err = client.ContainerUpdate(context.Background(), "container_id", container.UpdateConfig{
		Resources: container.Resources{
			CPUPeriod: 1,
		},
		RestartPolicy: container.RestartPolicy{
			Name: "always",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}
