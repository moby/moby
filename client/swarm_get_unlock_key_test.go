package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
)

func TestSwarmGetUnlockKeyError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.SwarmGetUnlockKey(context.Background())
	assert.Check(t, is.ErrorContains(err, "Error response from daemon: Server error"))
}

func TestSwarmGetUnlockKey(t *testing.T) {
	expectedURL := "/swarm/unlockkey"
	unlockKey := "SWMKEY-1-y6guTZNTwpQeTL5RhUfOsdBdXoQjiB2GADHSRJvbXeE"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != "GET" {
				return nil, fmt.Errorf("expected GET method, got %s", req.Method)
			}

			key := types.SwarmUnlockKeyResponse{
				UnlockKey: unlockKey,
			}

			b, err := json.Marshal(key)
			if err != nil {
				return nil, err
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewReader(b)),
			}, nil
		}),
	}

	resp, err := client.SwarmGetUnlockKey(context.Background())
	assert.NilError(t, err)
	assert.Check(t, is.Equal(unlockKey, resp.UnlockKey))
}
