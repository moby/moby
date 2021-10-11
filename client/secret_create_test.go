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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSecretCreateUnsupported(t *testing.T) {
	client := &Client{
		version: "1.24",
		client:  &http.Client{},
	}
	_, err := client.SecretCreate(context.Background(), swarm.SecretSpec{})
	assert.Check(t, is.Error(err, `"secret create" requires API version 1.25, but the Docker daemon API version is 1.24`))
}

func TestSecretCreateError(t *testing.T) {
	client := &Client{
		version: "1.25",
		client:  newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.SecretCreate(context.Background(), swarm.SecretSpec{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestSecretCreate(t *testing.T) {
	expectedURL := "/v1.25/secrets/create"
	client := &Client{
		version: "1.25",
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}
			b, err := json.Marshal(types.SecretCreateResponse{
				ID: "test_secret",
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}, nil
		}),
	}

	r, err := client.SecretCreate(context.Background(), swarm.SecretSpec{})
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != "test_secret" {
		t.Fatalf("expected `test_secret`, got %s", r.ID)
	}
}
