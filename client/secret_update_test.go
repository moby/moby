package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSecretUpdateUnsupported(t *testing.T) {
	client := &Client{
		version: "1.24",
		client:  &http.Client{},
	}
	err := client.SecretUpdate(context.Background(), "secret_id", swarm.Version{}, swarm.SecretSpec{})
	assert.Check(t, is.Error(err, `"secret update" requires API version 1.25, but the Docker daemon API version is 1.24`))
}

func TestSecretUpdateError(t *testing.T) {
	client := &Client{
		version: "1.25",
		client:  newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	err := client.SecretUpdate(context.Background(), "secret_id", swarm.Version{}, swarm.SecretSpec{})
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestSecretUpdate(t *testing.T) {
	expectedURL := "/v1.25/secrets/secret_id/update"

	client := &Client{
		version: "1.25",
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("body"))),
			}, nil
		}),
	}

	err := client.SecretUpdate(context.Background(), "secret_id", swarm.Version{}, swarm.SecretSpec{})
	if err != nil {
		t.Fatal(err)
	}
}
