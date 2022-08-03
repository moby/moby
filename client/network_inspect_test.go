package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestNetworkInspect(t *testing.T) {
	client, err := NewClientWithOpts(
		WithVersion("1.41"),
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				return nil, errors.New("expected GET method, got " + req.Method)
			}
			if req.URL.Path == "/v1.41/networks/" {
				return errorMock(http.StatusInternalServerError, "client should not make a request for empty IDs")(req)
			}
			if strings.HasPrefix(req.URL.Path, "/v1.41/networks/unknown") {
				return errorMock(http.StatusNotFound, "Error: No such network: unknown")(req)
			}
			if strings.HasPrefix(req.URL.Path, "/v1.41/networks/test-500-response") {
				return errorMock(http.StatusInternalServerError, "Server error")(req)
			}
			// other test-cases all use "network_id"
			if !strings.HasPrefix(req.URL.Path, "/v1.41/networks/network_id") {
				return nil, errors.New("expected URL '/v1.41/networks/network_id', got " + req.URL.Path)
			}
			if strings.Contains(req.URL.RawQuery, "scope=global") {
				return errorMock(http.StatusNotFound, "Error: No such network: network_id")(req)
			}
			var (
				content []byte
				err     error
			)
			if strings.Contains(req.URL.RawQuery, "verbose=true") {
				s := map[string]network.ServiceInfo{
					"web": {},
				}
				content, err = json.Marshal(types.NetworkResource{
					Name:     "mynetwork",
					Services: s,
				})
			} else {
				content, err = json.Marshal(types.NetworkResource{
					Name: "mynetwork",
				})
			}
			if err != nil {
				return nil, err
			}
			return &http.Response{
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(content)),
			}, nil
		})),
	)
	assert.NilError(t, err)

	t.Run("empty ID", func(t *testing.T) {
		// verify that the client does not create a request if the network-ID/name is empty.
		_, err := client.NetworkInspect(context.Background(), "", types.NetworkInspectOptions{})
		assert.Check(t, IsErrNotFound(err))
	})
	t.Run("no options", func(t *testing.T) {
		r, err := client.NetworkInspect(context.Background(), "network_id", types.NetworkInspectOptions{})
		assert.NilError(t, err)
		assert.Equal(t, r.Name, "mynetwork")
	})
	t.Run("verbose", func(t *testing.T) {
		r, err := client.NetworkInspect(context.Background(), "network_id", types.NetworkInspectOptions{Verbose: true})
		assert.NilError(t, err)
		assert.Equal(t, r.Name, "mynetwork")
		_, ok := r.Services["web"]
		if !ok {
			t.Fatalf("expected service `web` missing in the verbose output")
		}
	})
	t.Run("global scope", func(t *testing.T) {
		_, err := client.NetworkInspect(context.Background(), "network_id", types.NetworkInspectOptions{Scope: "global"})
		assert.ErrorContains(t, err, "Error: No such network: network_id")
		assert.Check(t, IsErrNotFound(err))
	})
	t.Run("unknown network", func(t *testing.T) {
		_, err := client.NetworkInspect(context.Background(), "unknown", types.NetworkInspectOptions{})
		assert.ErrorContains(t, err, "Error: No such network: unknown")
		assert.Check(t, IsErrNotFound(err))
	})
	t.Run("server error", func(t *testing.T) {
		// Just testing that an internal server error is converted correctly by the client
		_, err := client.NetworkInspect(context.Background(), "test-500-response", types.NetworkInspectOptions{})
		assert.Check(t, errdefs.IsSystem(err))
	})
}
