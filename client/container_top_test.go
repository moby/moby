package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestContainerTopError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)
	_, err = client.ContainerTop(context.Background(), "nothing", []string{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestContainerTop(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/containers/container_id/top"
	expectedProcesses := [][]string{
		{"p1", "p2"},
		{"p3"},
	}
	expectedTitles := []string{"title1", "title2"}

	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			query := req.URL.Query()
			args := query.Get("ps_args")
			if args != "arg1 arg2" {
				return nil, fmt.Errorf("args not set in URL query properly. Expected 'arg1 arg2', got %v", args)
			}

			b, err := json.Marshal(container.ContainerTopOKBody{
				Processes: [][]string{
					{"p1", "p2"},
					{"p3"},
				},
				Titles: []string{"title1", "title2"},
			})
			if err != nil {
				return nil, err
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}, nil
		})),
	)
	assert.NilError(t, err)

	processList, err := client.ContainerTop(context.Background(), "container_id", []string{"arg1", "arg2"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expectedProcesses, processList.Processes) {
		t.Fatalf("Processes: expected %v, got %v", expectedProcesses, processList.Processes)
	}
	if !reflect.DeepEqual(expectedTitles, processList.Titles) {
		t.Fatalf("Titles: expected %v, got %v", expectedTitles, processList.Titles)
	}
}
