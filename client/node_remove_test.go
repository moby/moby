package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestNodeRemoveError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	err = client.NodeRemove(context.Background(), "node_id", types.NodeRemoveOptions{Force: false})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestNodeRemove(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/nodes/node_id"

	removeCases := []struct {
		force         bool
		expectedForce string
	}{
		{
			expectedForce: "",
		},
		{
			force:         true,
			expectedForce: "1",
		},
	}

	for _, removeCase := range removeCases {
		client, err := NewClientWithOpts(
			WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, expectedURL) {
					return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
				}
				if req.Method != http.MethodDelete {
					return nil, fmt.Errorf("expected DELETE method, got %s", req.Method)
				}
				force := req.URL.Query().Get("force")
				if force != removeCase.expectedForce {
					return nil, fmt.Errorf("force not set in URL query properly. expected '%s', got %s", removeCase.expectedForce, force)
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte("body"))),
				}, nil
			})),
		)
		assert.NilError(t, err)

		err = client.NodeRemove(context.Background(), "node_id", types.NodeRemoveOptions{Force: removeCase.force})
		if err != nil {
			t.Fatal(err)
		}
	}
}
