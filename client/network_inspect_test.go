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
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNetworkInspectError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.NetworkInspect(context.Background(), "nothing", types.NetworkInspectOptions{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestNetworkInspectNotFoundError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "missing")),
	}

	_, err := client.NetworkInspect(context.Background(), "unknown", types.NetworkInspectOptions{})
	assert.Check(t, is.Error(err, "Error: No such network: unknown"))
	assert.Check(t, IsErrNotFound(err))
}

func TestNetworkInspectWithEmptyID(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("should not make request")
		}),
	}
	_, _, err := client.NetworkInspectWithRaw(context.Background(), "", types.NetworkInspectOptions{})
	if !IsErrNotFound(err) {
		t.Fatalf("Expected NotFoundError, got %v", err)
	}
}

func TestNetworkInspect(t *testing.T) {
	expectedURL := "/networks/network_id"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodGet {
				return nil, fmt.Errorf("expected GET method, got %s", req.Method)
			}

			var (
				content []byte
				err     error
			)
			if strings.Contains(req.URL.RawQuery, "scope=global") {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(bytes.NewReader(content)),
				}, nil
			}

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
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(content)),
			}, nil
		}),
	}

	r, err := client.NetworkInspect(context.Background(), "network_id", types.NetworkInspectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "mynetwork" {
		t.Fatalf("expected `mynetwork`, got %s", r.Name)
	}

	r, err = client.NetworkInspect(context.Background(), "network_id", types.NetworkInspectOptions{Verbose: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "mynetwork" {
		t.Fatalf("expected `mynetwork`, got %s", r.Name)
	}
	_, ok := r.Services["web"]
	if !ok {
		t.Fatalf("expected service `web` missing in the verbose output")
	}

	_, err = client.NetworkInspect(context.Background(), "network_id", types.NetworkInspectOptions{Scope: "global"})
	assert.Check(t, is.Error(err, "Error: No such network: network_id"))
}
