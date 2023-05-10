package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerStatPathError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ContainerStatPath(context.Background(), "container_id", "path")
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestContainerStatPathNotFoundError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "Not found")),
	}
	_, err := client.ContainerStatPath(context.Background(), "container_id", "path")
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
}

func TestContainerStatPathNoHeaderError(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	}
	_, err := client.ContainerStatPath(context.Background(), "container_id", "path/to/file")
	if err == nil {
		t.Fatalf("expected an error, got nothing")
	}
}

func TestContainerStatPath(t *testing.T) {
	expectedURL := "/containers/container_id/archive"
	expectedPath := "path/to/file"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodHead {
				return nil, fmt.Errorf("expected HEAD method, got %s", req.Method)
			}
			query := req.URL.Query()
			path := query.Get("path")
			if path != expectedPath {
				return nil, fmt.Errorf("path not set in URL query properly")
			}
			content, err := json.Marshal(types.ContainerPathStat{
				Name: "name",
				Mode: 0700,
			})
			if err != nil {
				return nil, err
			}
			base64PathStat := base64.StdEncoding.EncodeToString(content)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
				Header: http.Header{
					"X-Docker-Container-Path-Stat": []string{base64PathStat},
				},
			}, nil
		}),
	}
	stat, err := client.ContainerStatPath(context.Background(), "container_id", expectedPath)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Name != "name" {
		t.Fatalf("expected container path stat name to be 'name', got '%s'", stat.Name)
	}
	if stat.Mode != 0700 {
		t.Fatalf("expected container path stat mode to be 0700, got '%v'", stat.Mode)
	}
}

func TestCopyToContainerError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	err := client.CopyToContainer(context.Background(), "container_id", "path/to/file", bytes.NewReader([]byte("")), types.CopyToContainerOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestCopyToContainerNotFoundError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "Not found")),
	}
	err := client.CopyToContainer(context.Background(), "container_id", "path/to/file", bytes.NewReader([]byte("")), types.CopyToContainerOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
}

// TestCopyToContainerEmptyResponse verifies that no error is returned when a
// "204 No Content" is returned by the API.
func TestCopyToContainerEmptyResponse(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNoContent, "No content")),
	}
	err := client.CopyToContainer(context.Background(), "container_id", "path/to/file", bytes.NewReader([]byte("")), types.CopyToContainerOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCopyToContainer(t *testing.T) {
	expectedURL := "/containers/container_id/archive"
	expectedPath := "path/to/file"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodPut {
				return nil, fmt.Errorf("expected PUT method, got %s", req.Method)
			}
			query := req.URL.Query()
			path := query.Get("path")
			if path != expectedPath {
				return nil, fmt.Errorf("path not set in URL query properly, expected '%s', got %s", expectedPath, path)
			}
			noOverwriteDirNonDir := query.Get("noOverwriteDirNonDir")
			if noOverwriteDirNonDir != "true" {
				return nil, fmt.Errorf("noOverwriteDirNonDir not set in URL query properly, expected true, got %s", noOverwriteDirNonDir)
			}

			content, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			if err := req.Body.Close(); err != nil {
				return nil, err
			}
			if string(content) != "content" {
				return nil, fmt.Errorf("expected content to be 'content', got %s", string(content))
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	}
	err := client.CopyToContainer(context.Background(), "container_id", expectedPath, bytes.NewReader([]byte("content")), types.CopyToContainerOptions{
		AllowOverwriteDirWithFile: false,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCopyFromContainerError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, _, err := client.CopyFromContainer(context.Background(), "container_id", "path/to/file")
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestCopyFromContainerNotFoundError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "Not found")),
	}
	_, _, err := client.CopyFromContainer(context.Background(), "container_id", "path/to/file")
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
}

// TestCopyFromContainerEmptyResponse verifies that no error is returned when a
// "204 No Content" is returned by the API.
func TestCopyFromContainerEmptyResponse(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			content, err := json.Marshal(types.ContainerPathStat{
				Name: "path/to/file",
				Mode: 0700,
			})
			if err != nil {
				return nil, err
			}
			base64PathStat := base64.StdEncoding.EncodeToString(content)
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Header: http.Header{
					"X-Docker-Container-Path-Stat": []string{base64PathStat},
				},
			}, nil
		}),
	}
	_, _, err := client.CopyFromContainer(context.Background(), "container_id", "path/to/file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCopyFromContainerNoHeaderError(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	}
	_, _, err := client.CopyFromContainer(context.Background(), "container_id", "path/to/file")
	if err == nil {
		t.Fatalf("expected an error, got nothing")
	}
}

func TestCopyFromContainer(t *testing.T) {
	expectedURL := "/containers/container_id/archive"
	expectedPath := "path/to/file"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodGet {
				return nil, fmt.Errorf("expected GET method, got %s", req.Method)
			}
			query := req.URL.Query()
			path := query.Get("path")
			if path != expectedPath {
				return nil, fmt.Errorf("path not set in URL query properly, expected '%s', got %s", expectedPath, path)
			}

			headercontent, err := json.Marshal(types.ContainerPathStat{
				Name: "name",
				Mode: 0700,
			})
			if err != nil {
				return nil, err
			}
			base64PathStat := base64.StdEncoding.EncodeToString(headercontent)

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("content"))),
				Header: http.Header{
					"X-Docker-Container-Path-Stat": []string{base64PathStat},
				},
			}, nil
		}),
	}
	r, stat, err := client.CopyFromContainer(context.Background(), "container_id", expectedPath)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Name != "name" {
		t.Fatalf("expected container path stat name to be 'name', got '%s'", stat.Name)
	}
	if stat.Mode != 0700 {
		t.Fatalf("expected container path stat mode to be 0700, got '%v'", stat.Mode)
	}
	content, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if string(content) != "content" {
		t.Fatalf("expected content to be 'content', got %s", string(content))
	}
}
