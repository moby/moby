package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/registry"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageSearchAnyError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ImageSearch(context.Background(), "some-image", ImageSearchOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestImageSearchStatusUnauthorizedError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")),
	}
	_, err := client.ImageSearch(context.Background(), "some-image", ImageSearchOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsUnauthorized))
}

func TestImageSearchWithUnauthorizedErrorAndPrivilegeFuncError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")),
	}
	privilegeFunc := func(_ context.Context) (string, error) {
		return "", errors.New("Error requesting privilege")
	}
	_, err := client.ImageSearch(context.Background(), "some-image", ImageSearchOptions{
		PrivilegeFunc: privilegeFunc,
	})
	assert.Check(t, is.Error(err, "Error requesting privilege"))
}

func TestImageSearchWithUnauthorizedErrorAndAnotherUnauthorizedError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")),
	}
	privilegeFunc := func(_ context.Context) (string, error) {
		return "a-auth-header", nil
	}
	_, err := client.ImageSearch(context.Background(), "some-image", ImageSearchOptions{
		PrivilegeFunc: privilegeFunc,
	})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsUnauthorized))
}

func TestImageSearchWithPrivilegedFuncNoError(t *testing.T) {
	expectedURL := "/images/search"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			auth := req.Header.Get(registry.AuthHeader)
			if auth == "NotValid" {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(bytes.NewReader([]byte("Invalid credentials"))),
				}, nil
			}
			if auth != "IAmValid" {
				return nil, fmt.Errorf("invalid auth header: expected 'IAmValid', got %s", auth)
			}
			query := req.URL.Query()
			term := query.Get("term")
			if term != "some-image" {
				return nil, fmt.Errorf("term not set in URL query properly. Expected 'some-image', got %s", term)
			}
			content, err := json.Marshal([]registry.SearchResult{
				{
					Name: "anything",
				},
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(content)),
			}, nil
		}),
	}
	privilegeFunc := func(_ context.Context) (string, error) {
		return "IAmValid", nil
	}
	results, err := client.ImageSearch(context.Background(), "some-image", ImageSearchOptions{
		RegistryAuth:  "NotValid",
		PrivilegeFunc: privilegeFunc,
	})
	assert.NilError(t, err)
	assert.Check(t, is.Len(results, 1))
}

func TestImageSearchWithoutErrors(t *testing.T) {
	const expectedURL = "/images/search"
	const expectedFilters = `{"is-automated":{"true":true},"stars":{"3":true}}`

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			query := req.URL.Query()
			term := query.Get("term")
			if term != "some-image" {
				return nil, fmt.Errorf("term not set in URL query properly. Expected 'some-image', got %s", term)
			}
			fltrs := query.Get("filters")
			if fltrs != expectedFilters {
				return nil, fmt.Errorf("filters not set in URL query properly. Expected '%s', got %s", expectedFilters, fltrs)
			}
			content, err := json.Marshal([]registry.SearchResult{
				{
					Name: "anything",
				},
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(content)),
			}, nil
		}),
	}
	results, err := client.ImageSearch(context.Background(), "some-image", ImageSearchOptions{
		Filters: filters.NewArgs(
			filters.Arg("is-automated", "true"),
			filters.Arg("stars", "3"),
		),
	})
	assert.NilError(t, err)
	assert.Check(t, is.Len(results, 1))
}
