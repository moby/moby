package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSecretInspectUnsupported(t *testing.T) {
	client := &Client{
		version: "1.24",
		client:  &http.Client{},
	}
	_, _, err := client.SecretInspectWithRaw(context.Background(), "nothing")
	assert.Check(t, is.Error(err, `"secret inspect" requires API version 1.25, but the Docker daemon API version is 1.24`))
}

func TestSecretInspectError(t *testing.T) {
	client := &Client{
		version: "1.25",
		client:  newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, _, err := client.SecretInspectWithRaw(context.Background(), "nothing")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestSecretInspectSecretNotFound(t *testing.T) {
	client := &Client{
		version: "1.25",
		client:  newMockClient(errorMock(http.StatusNotFound, "Server error")),
	}

	_, _, err := client.SecretInspectWithRaw(context.Background(), "unknown")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestSecretInspectWithEmptyID(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("should not make request")
		}),
	}
	_, _, err := client.SecretInspectWithRaw(context.Background(), "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, _, err = client.SecretInspectWithRaw(context.Background(), "    ")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestSecretInspect(t *testing.T) {
	expectedURL := "/v1.25/secrets/secret_id"
	client := &Client{
		version: "1.25",
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
				Body:       io.NopCloser(bytes.NewReader(content)),
			}, nil
		}),
	}

	secretInspect, _, err := client.SecretInspectWithRaw(context.Background(), "secret_id")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(secretInspect.ID, "secret_id"))
}
