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

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSecretInspectUnsupported(t *testing.T) {
	client, err := NewClientWithOpts(
		WithVersion("1.24"),
	)
	assert.NilError(t, err)
	_, _, err = client.SecretInspectWithRaw(context.Background(), "nothing")
	assert.Check(t, is.Error(err, `"secret inspect" requires API version 1.25, but the Docker daemon API version is 1.24`))
}

func TestSecretInspectError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithVersion("1.25"),
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	_, _, err = client.SecretInspectWithRaw(context.Background(), "nothing")
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestSecretInspectSecretNotFound(t *testing.T) {
	client, err := NewClientWithOpts(
		WithVersion("1.25"),
		WithHTTPClient(newMockClient(errorMock(http.StatusNotFound, "Server error"))),
	)
	assert.NilError(t, err)

	_, _, err = client.SecretInspectWithRaw(context.Background(), "unknown")
	if err == nil || !IsErrNotFound(err) {
		t.Fatalf("expected a secretNotFoundError error, got %v", err)
	}
}

func TestSecretInspectWithEmptyID(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("should not make request")
		})),
	)
	assert.NilError(t, err)
	_, _, err = client.SecretInspectWithRaw(context.Background(), "")
	if !IsErrNotFound(err) {
		t.Fatalf("Expected NotFoundError, got %v", err)
	}
}

func TestSecretInspect(t *testing.T) {
	expectedURL := "/v1.25/secrets/secret_id"
	client, err := NewClientWithOpts(
		WithVersion("1.25"),
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			content, err := json.Marshal(swarm.Secret{
				ID: "secret_id",
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(content)),
			}, nil
		})),
	)
	assert.NilError(t, err)

	secretInspect, _, err := client.SecretInspectWithRaw(context.Background(), "secret_id")
	if err != nil {
		t.Fatal(err)
	}
	if secretInspect.ID != "secret_id" {
		t.Fatalf("expected `secret_id`, got %s", secretInspect.ID)
	}
}
