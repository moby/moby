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

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerUpdateError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ContainerUpdate(context.Background(), "nothing", container.UpdateConfig{})
	assert.Check(t, is.ErrorType(err, errdefs.IsInternal))
}

func TestContainerUpdate(t *testing.T) {
	expectedURL := "/containers/container_id/update"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
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
		}),
	}

	_, err := client.ContainerUpdate(context.Background(), "container_id", container.UpdateConfig{
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
