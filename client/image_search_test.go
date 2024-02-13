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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageSearchAnyError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ImageSearch(context.Background(), "some-image", types.ImageSearchOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestImageSearchStatusUnauthorizedError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")),
	}
	_, err := client.ImageSearch(context.Background(), "some-image", types.ImageSearchOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsUnauthorized))
}

func TestImageSearchWithUnauthorizedErrorAndPrivilegeFuncError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")),
	}
	privilegeFunc := func() (string, error) {
		return "", fmt.Errorf("Error requesting privilege")
	}
	_, err := client.ImageSearch(context.Background(), "some-image", types.ImageSearchOptions{
		PrivilegeFunc: privilegeFunc,
	})
	if err == nil || err.Error() != "Error requesting privilege" {
		t.Fatalf("expected an error requesting privilege, got %v", err)
	}
}

func TestImageSearchWithUnauthorizedErrorAndAnotherUnauthorizedError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")),
	}
	privilegeFunc := func() (string, error) {
		return "a-auth-header", nil
	}
	_, err := client.ImageSearch(context.Background(), "some-image", types.ImageSearchOptions{
		PrivilegeFunc: privilegeFunc,
	})
	assert.Check(t, is.ErrorType(err, errdefs.IsUnauthorized))
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
	results, err := client.ImageSearch(context.Background(), "some-image", types.ImageSearchOptions{
		Filters: filters.NewArgs(
			filters.Arg("is-automated", "true"),
			filters.Arg("stars", "3"),
		),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected a result, got %v", results)
	}
}
