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
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerStatPathError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerStatPath(context.Background(), "container_id", ContainerStatPathOptions{Path: "path"})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerStatPath(context.Background(), "", ContainerStatPathOptions{Path: "path"})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerStatPath(context.Background(), "    ", ContainerStatPathOptions{Path: "path"})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerStatPathNotFoundError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusNotFound, "Not found")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerStatPath(context.Background(), "container_id", ContainerStatPathOptions{Path: "path"})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestContainerStatPathNoHeaderError(t *testing.T) {
	client, err := New(
		WithMockClient(mockResponse(http.StatusOK, nil, "")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerStatPath(context.Background(), "container_id", ContainerStatPathOptions{Path: "path/to/file"})
	assert.Check(t, err != nil, "expected an error, got nothing")
}

func TestContainerStatPath(t *testing.T) {
	const (
		expectedURL  = "/containers/container_id/archive"
		expectedPath = "path/to/file"
	)
	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodHead, expectedURL); err != nil {
				return nil, err
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
			hdr := http.Header{
				"X-Docker-Container-Path-Stat": []string{base64PathStat},
			}
			return mockResponse(http.StatusOK, hdr, "")(req)
		}),
	)
	assert.NilError(t, err)
	res, err := client.ContainerStatPath(context.Background(), "container_id", ContainerStatPathOptions{Path: expectedPath})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(res.Stat.Name, "name"))
	assert.Check(t, is.Equal(res.Stat.Mode, os.FileMode(0o700)))
}

func TestCopyToContainerError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.CopyToContainer(context.Background(), "container_id", CopyToContainerOptions{
		DestinationPath: "path/to/file",
		Content:         bytes.NewReader([]byte("")),
	})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.CopyToContainer(context.Background(), "", CopyToContainerOptions{
		DestinationPath: "path/to/file",
		Content:         bytes.NewReader([]byte("")),
	})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.CopyToContainer(context.Background(), "    ", CopyToContainerOptions{
		DestinationPath: "path/to/file",
		Content:         bytes.NewReader([]byte("")),
	})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestCopyToContainerNotFoundError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusNotFound, "Not found")),
	)
	assert.NilError(t, err)

	_, err = client.CopyToContainer(context.Background(), "container_id", CopyToContainerOptions{
		DestinationPath: "path/to/file",
		Content:         bytes.NewReader([]byte("")),
	})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

// TestCopyToContainerEmptyResponse verifies that no error is returned when a
// "204 No Content" is returned by the API.
func TestCopyToContainerEmptyResponse(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusNoContent, "No content")),
	)
	assert.NilError(t, err)

	_, err = client.CopyToContainer(context.Background(), "container_id", CopyToContainerOptions{
		DestinationPath: "path/to/file",
		Content:         bytes.NewReader([]byte("")),
	})
	assert.NilError(t, err)
}

func TestCopyToContainer(t *testing.T) {
	const (
		expectedURL  = "/containers/container_id/archive"
		expectedPath = "path/to/file"
	)
	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodPut, expectedURL); err != nil {
				return nil, err
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

			return mockResponse(http.StatusOK, nil, "")(req)
		}),
	)
	assert.NilError(t, err)

	_, err = client.CopyToContainer(context.Background(), "container_id", CopyToContainerOptions{
		DestinationPath:           expectedPath,
		Content:                   bytes.NewReader([]byte("content")),
		AllowOverwriteDirWithFile: false,
	})
	assert.NilError(t, err)
}

func TestCopyFromContainerError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.CopyFromContainer(context.Background(), "container_id", CopyFromContainerOptions{SourcePath: "path/to/file"})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.CopyFromContainer(context.Background(), "", CopyFromContainerOptions{SourcePath: "path/to/file"})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.CopyFromContainer(context.Background(), "    ", CopyFromContainerOptions{SourcePath: "path/to/file"})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestCopyFromContainerNotFoundError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusNotFound, "Not found")),
	)
	assert.NilError(t, err)

	_, err = client.CopyFromContainer(context.Background(), "container_id", CopyFromContainerOptions{SourcePath: "path/to/file"})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

// TestCopyFromContainerEmptyResponse verifies that no error is returned when a
// "204 No Content" is returned by the API.
func TestCopyFromContainerEmptyResponse(t *testing.T) {
	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			content, err := json.Marshal(container.PathStat{
				Name: "path/to/file",
				Mode: 0o700,
			})
			if err != nil {
				return nil, err
			}
			base64PathStat := base64.StdEncoding.EncodeToString(content)
			hdr := http.Header{
				"X-Docker-Container-Path-Stat": []string{base64PathStat},
			}
			return mockResponse(http.StatusNoContent, hdr, "")(req)
		}),
	)
	assert.NilError(t, err)

	_, err = client.CopyFromContainer(context.Background(), "container_id", CopyFromContainerOptions{SourcePath: "path/to/file"})
	assert.NilError(t, err)
}

func TestCopyFromContainerNoHeaderError(t *testing.T) {
	client, err := New(
		WithMockClient(mockResponse(http.StatusOK, nil, "")),
	)
	assert.NilError(t, err)

	_, err = client.CopyFromContainer(context.Background(), "container_id", CopyFromContainerOptions{SourcePath: "path/to/file"})
	assert.Check(t, err != nil, "expected an error, got nothing")
}

func TestCopyFromContainer(t *testing.T) {
	const (
		expectedURL  = "/containers/container_id/archive"
		expectedPath = "path/to/file"
	)
	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
				return nil, err
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
			hdr := http.Header{
				"X-Docker-Container-Path-Stat": []string{base64PathStat},
			}
			return mockResponse(http.StatusOK, hdr, "content")(req)
		}),
	)
	assert.NilError(t, err)
	res2, err := client.CopyFromContainer(context.Background(), "container_id", CopyFromContainerOptions{SourcePath: expectedPath})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(res2.Stat.Name, "name"))
	assert.Check(t, is.Equal(res2.Stat.Mode, os.FileMode(0o700)))

	content, err := io.ReadAll(res2.Content)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(string(content), "content"))
	assert.NilError(t, res2.Content.Close())
}
