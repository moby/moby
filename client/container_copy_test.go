package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerStatPathError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ContainerStatPath(context.Background(), "container_id", "path")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerStatPath(context.Background(), "", "path")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerStatPath(context.Background(), "    ", "path")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerStatPathNotFoundError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "Not found")),
	}
	_, err := client.ContainerStatPath(context.Background(), "container_id", "path")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
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
	assert.Check(t, err != nil, "expected an error, got nothing")
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
				return nil, errors.New("path not set in URL query properly")
			}
			content, err := json.Marshal(container.PathStat{
				Name: "name",
				Mode: 0o700,
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
	assert.NilError(t, err)
	assert.Check(t, is.Equal(stat.Name, "name"))
	assert.Check(t, is.Equal(stat.Mode, os.FileMode(0o700)))
}

func TestCopyToContainerError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	err := client.CopyToContainer(context.Background(), "container_id", "path/to/file", bytes.NewReader([]byte("")), container.CopyToContainerOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.CopyToContainer(context.Background(), "", "path/to/file", bytes.NewReader([]byte("")), container.CopyToContainerOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.CopyToContainer(context.Background(), "    ", "path/to/file", bytes.NewReader([]byte("")), container.CopyToContainerOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestCopyToContainerNotFoundError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "Not found")),
	}
	err := client.CopyToContainer(context.Background(), "container_id", "path/to/file", bytes.NewReader([]byte("")), container.CopyToContainerOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

// TestCopyToContainerEmptyResponse verifies that no error is returned when a
// "204 No Content" is returned by the API.
func TestCopyToContainerEmptyResponse(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNoContent, "No content")),
	}
	err := client.CopyToContainer(context.Background(), "container_id", "path/to/file", bytes.NewReader([]byte("")), container.CopyToContainerOptions{})
	assert.NilError(t, err)
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
	err := client.CopyToContainer(context.Background(), "container_id", expectedPath, bytes.NewReader([]byte("content")), container.CopyToContainerOptions{
		AllowOverwriteDirWithFile: false,
	})
	assert.NilError(t, err)
}

func TestCopyFromContainerError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, _, err := client.CopyFromContainer(context.Background(), "container_id", "path/to/file")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, _, err = client.CopyFromContainer(context.Background(), "", "path/to/file")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, _, err = client.CopyFromContainer(context.Background(), "    ", "path/to/file")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestCopyFromContainerNotFoundError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "Not found")),
	}
	_, _, err := client.CopyFromContainer(context.Background(), "container_id", "path/to/file")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

// TestCopyFromContainerEmptyResponse verifies that no error is returned when a
// "204 No Content" is returned by the API.
func TestCopyFromContainerEmptyResponse(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			content, err := json.Marshal(container.PathStat{
				Name: "path/to/file",
				Mode: 0o700,
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
	assert.NilError(t, err)
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
	assert.Check(t, err != nil, "expected an error, got nothing")
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

			headercontent, err := json.Marshal(container.PathStat{
				Name: "name",
				Mode: 0o700,
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
	assert.NilError(t, err)
	assert.Check(t, is.Equal(stat.Name, "name"))
	assert.Check(t, is.Equal(stat.Mode, os.FileMode(0o700)))

	content, err := io.ReadAll(r)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(string(content), "content"))
	assert.NilError(t, r.Close())
}
