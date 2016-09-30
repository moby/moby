package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"golang.org/x/net/context"
)

func TestContainerCreateError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ContainerCreate(context.Background(), nil, nil, nil, "nothing")
	if err == nil || err.Error() != "Error response from daemon: Server error" {
		t.Fatalf("expected a Server Error, got %v", err)
	}

	// 404 doesn't automagitally means an unknown image
	client = &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "Server error")),
	}
	_, err = client.ContainerCreate(context.Background(), nil, nil, nil, "nothing")
	if err == nil || err.Error() != "Error response from daemon: Server error" {
		t.Fatalf("expected a Server Error, got %v", err)
	}
}

func TestContainerCreateImageNotFound(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "No such image")),
	}
	_, err := client.ContainerCreate(context.Background(), &container.Config{Image: "unknown_image"}, nil, nil, "unknown")
	if err == nil || !IsErrImageNotFound(err) {
		t.Fatalf("expected an imageNotFound error, got %v", err)
	}
}

func TestContainerCreateWithName(t *testing.T) {
	expectedURL := "/containers/create"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			name := req.URL.Query().Get("name")
			if name != "container_name" {
				return nil, fmt.Errorf("container name not set in URL query properly. Expected `container_name`, got %s", name)
			}
			b, err := json.Marshal(types.ContainerCreateResponse{
				ID: "container_id",
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewReader(b)),
			}, nil
		}),
	}

	r, err := client.ContainerCreate(context.Background(), nil, nil, nil, "container_name")
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != "container_id" {
		t.Fatalf("expected `container_id`, got %s", r.ID)
	}
}
