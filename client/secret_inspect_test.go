package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"golang.org/x/net/context"
)

func TestSecretInspectError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, _, err := client.SecretInspectWithRaw(context.Background(), "nothing")
	if err == nil || err.Error() != "Error response from daemon: Server error" {
		t.Fatalf("expected a Server Error, got %v", err)
	}
}

func TestSecretInspectSecretNotFound(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "Server error")),
	}

	_, _, err := client.SecretInspectWithRaw(context.Background(), "unknown")
	if err == nil || !IsErrSecretNotFound(err) {
		t.Fatalf("expected a secretNotFoundError error, got %v", err)
	}
}

func TestSecretInspect(t *testing.T) {
	expectedURL := "/secrets/secret_id"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
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
				Body:       ioutil.NopCloser(bytes.NewReader(content)),
			}, nil
		}),
	}

	secretInspect, _, err := client.SecretInspectWithRaw(context.Background(), "secret_id")
	if err != nil {
		t.Fatal(err)
	}
	if secretInspect.ID != "secret_id" {
		t.Fatalf("expected `secret_id`, got %s", secretInspect.ID)
	}
}
