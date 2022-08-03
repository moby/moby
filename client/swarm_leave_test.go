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
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestSwarmLeaveError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	err = client.SwarmLeave(context.Background(), false)
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestSwarmLeave(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/swarm/leave"

	leaveCases := []struct {
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

	for _, leaveCase := range leaveCases {
		client, err := NewClientWithOpts(
			WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, expectedURL) {
					return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
				}
				if req.Method != http.MethodPost {
					return nil, fmt.Errorf("expected POST method, got %s", req.Method)
				}
				force := req.URL.Query().Get("force")
				if force != leaveCase.expectedForce {
					return nil, fmt.Errorf("force not set in URL query properly. expected '%s', got %s", leaveCase.expectedForce, force)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte(""))),
				}, nil
			})),
		)
		assert.NilError(t, err)

		err = client.SwarmLeave(context.Background(), leaveCase.force)
		if err != nil {
			t.Fatal(err)
		}
	}
}
