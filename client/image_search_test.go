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

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestImageSearchAnyError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)
	_, err = client.ImageSearch(context.Background(), "some-image", types.ImageSearchOptions{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestImageSearchStatusUnauthorizedError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error"))),
	)
	assert.NilError(t, err)
	_, err = client.ImageSearch(context.Background(), "some-image", types.ImageSearchOptions{})
	if !errdefs.IsUnauthorized(err) {
		t.Fatalf("expected a Unauthorized Error, got %[1]T: %[1]v", err)
	}
}

func TestImageSearchWithUnauthorizedErrorAndPrivilegeFuncError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error"))),
	)
	assert.NilError(t, err)
	privilegeFunc := func() (string, error) {
		return "", fmt.Errorf("Error requesting privilege")
	}
	_, err = client.ImageSearch(context.Background(), "some-image", types.ImageSearchOptions{
		PrivilegeFunc: privilegeFunc,
	})
	if err == nil || err.Error() != "Error requesting privilege" {
		t.Fatalf("expected an error requesting privilege, got %v", err)
	}
}

func TestImageSearchWithUnauthorizedErrorAndAnotherUnauthorizedError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error"))),
	)
	assert.NilError(t, err)
	privilegeFunc := func() (string, error) {
		return "a-auth-header", nil
	}
	_, err = client.ImageSearch(context.Background(), "some-image", types.ImageSearchOptions{
		PrivilegeFunc: privilegeFunc,
	})
	if !errdefs.IsUnauthorized(err) {
		t.Fatalf("expected a Unauthorized Error, got %[1]T: %[1]v", err)
	}
}

func TestImageSearchWithPrivilegedFuncNoError(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/images/search"
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			auth := req.Header.Get("X-Registry-Auth")
			if auth == "NotValid" {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(bytes.NewReader([]byte("Invalid credentials"))),
				}, nil
			}
			if auth != "IAmValid" {
				return nil, fmt.Errorf("Invalid auth header : expected 'IAmValid', got %s", auth)
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
		})),
	)
	assert.NilError(t, err)
	privilegeFunc := func() (string, error) {
		return "IAmValid", nil
	}
	results, err := client.ImageSearch(context.Background(), "some-image", types.ImageSearchOptions{
		RegistryAuth:  "NotValid",
		PrivilegeFunc: privilegeFunc,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %v", results)
	}
}

func TestImageSearchWithoutErrors(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/images/search"
	filterArgs := filters.NewArgs()
	filterArgs.Add("is-automated", "true")
	filterArgs.Add("stars", "3")

	expectedFilters := `{"is-automated":{"true":true},"stars":{"3":true}}`

	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			query := req.URL.Query()
			term := query.Get("term")
			if term != "some-image" {
				return nil, fmt.Errorf("term not set in URL query properly. Expected 'some-image', got %s", term)
			}
			filters := query.Get("filters")
			if filters != expectedFilters {
				return nil, fmt.Errorf("filters not set in URL query properly. Expected '%s', got %s", expectedFilters, filters)
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
		})),
	)
	assert.NilError(t, err)
	results, err := client.ImageSearch(context.Background(), "some-image", types.ImageSearchOptions{
		Filters: filterArgs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected a result, got %v", results)
	}
}
