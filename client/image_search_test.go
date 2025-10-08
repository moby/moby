package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/registry"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageSearchAnyError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.ImageSearch(context.Background(), "some-image", ImageSearchOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestImageSearchStatusUnauthorizedError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")))
	assert.NilError(t, err)
	_, err = client.ImageSearch(context.Background(), "some-image", ImageSearchOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsUnauthorized))
}

func TestImageSearchWithUnauthorizedErrorAndPrivilegeFuncError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")))
	assert.NilError(t, err)
	privilegeFunc := func(_ context.Context) (string, error) {
		return "", errors.New("Error requesting privilege")
	}
	_, err = client.ImageSearch(context.Background(), "some-image", ImageSearchOptions{
		PrivilegeFunc: privilegeFunc,
	})
	assert.Check(t, is.Error(err, "Error requesting privilege"))
}

func TestImageSearchWithUnauthorizedErrorAndAnotherUnauthorizedError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")))
	assert.NilError(t, err)
	privilegeFunc := func(_ context.Context) (string, error) {
		return "a-auth-header", nil
	}
	_, err = client.ImageSearch(context.Background(), "some-image", ImageSearchOptions{
		PrivilegeFunc: privilegeFunc,
	})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsUnauthorized))
}

func TestImageSearchWithPrivilegedFuncNoError(t *testing.T) {
	const expectedURL = "/images/search"
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
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
	}))
	assert.NilError(t, err)
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

	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
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
	}))
	assert.NilError(t, err)
	results, err := client.ImageSearch(context.Background(), "some-image", ImageSearchOptions{
		Filters: make(Filters).Add("is-automated", "true").Add("stars", "3"),
	})
	assert.NilError(t, err)
	assert.Check(t, is.Len(results, 1))
}
