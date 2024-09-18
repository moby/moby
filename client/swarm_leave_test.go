package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSwarmLeaveError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	err := client.SwarmLeave(context.Background(), false)
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestSwarmLeave(t *testing.T) {
	expectedURL := "/swarm/leave"

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
		client := &Client{
			client: newMockClient(func(req *http.Request) (*http.Response, error) {
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
			}),
		}

		err := client.SwarmLeave(context.Background(), leaveCase.force)
		if err != nil {
			t.Fatal(err)
		}
	}
}
