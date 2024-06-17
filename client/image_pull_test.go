package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImagePullReferenceParseError(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, nil
		}),
	}
	// An empty reference is an invalid reference
	_, err := client.ImagePull(context.Background(), "", image.PullOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid reference format") {
		t.Fatalf("expected an error, got %v", err)
	}
}

func TestImagePullAnyError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ImagePull(context.Background(), "myimage", image.PullOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestImagePullStatusUnauthorizedError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")),
	}
	_, err := client.ImagePull(context.Background(), "myimage", image.PullOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsUnauthorized))
}

func TestImagePullWithUnauthorizedErrorAndPrivilegeFuncError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")),
	}
	privilegeFunc := func(_ context.Context) (string, error) {
		return "", fmt.Errorf("Error requesting privilege")
	}
	_, err := client.ImagePull(context.Background(), "myimage", image.PullOptions{
		PrivilegeFunc: privilegeFunc,
	})
	if err == nil || err.Error() != "Error requesting privilege" {
		t.Fatalf("expected an error requesting privilege, got %v", err)
	}
}

func TestImagePullWithUnauthorizedErrorAndAnotherUnauthorizedError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")),
	}
	privilegeFunc := func(_ context.Context) (string, error) {
		return "a-auth-header", nil
	}
	_, err := client.ImagePull(context.Background(), "myimage", image.PullOptions{
		PrivilegeFunc: privilegeFunc,
	})
	assert.Check(t, is.ErrorType(err, errdefs.IsUnauthorized))
}

func TestImagePullWithPrivilegedFuncNoError(t *testing.T) {
	expectedURL := "/images/create"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			auth := req.Header.Get(registry.AuthHeader)
			if auth == "NotValid" {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(bytes.NewReader([]byte("Invalid credentials"))),
				}, nil
			}
			if auth != "IAmValid" {
				return nil, fmt.Errorf("invalid auth header: expected %s, got %s", "IAmValid", auth)
			}
			query := req.URL.Query()
			fromImage := query.Get("fromImage")
			if fromImage != "myimage" {
				return nil, fmt.Errorf("fromimage not set in URL query properly. Expected '%s', got %s", "myimage", fromImage)
			}
			tag := query.Get("tag")
			if tag != "latest" {
				return nil, fmt.Errorf("tag not set in URL query properly. Expected '%s', got %s", "latest", tag)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("hello world"))),
			}, nil
		}),
	}
	privilegeFunc := func(_ context.Context) (string, error) {
		return "IAmValid", nil
	}
	resp, err := client.ImagePull(context.Background(), "myimage", image.PullOptions{
		RegistryAuth:  "NotValid",
		PrivilegeFunc: privilegeFunc,
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello world" {
		t.Fatalf("expected 'hello world', got %s", string(body))
	}
}

func TestImagePullWithoutErrors(t *testing.T) {
	expectedURL := "/images/create"
	expectedOutput := "hello world"
	pullCases := []struct {
		all           bool
		reference     string
		expectedImage string
		expectedTag   string
	}{
		{
			all:           false,
			reference:     "myimage",
			expectedImage: "myimage",
			expectedTag:   "latest",
		},
		{
			all:           false,
			reference:     "myimage:tag",
			expectedImage: "myimage",
			expectedTag:   "tag",
		},
		{
			all:           true,
			reference:     "myimage",
			expectedImage: "myimage",
			expectedTag:   "",
		},
		{
			all:           true,
			reference:     "myimage:anything",
			expectedImage: "myimage",
			expectedTag:   "",
		},
	}
	for _, pullCase := range pullCases {
		client := &Client{
			client: newMockClient(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, expectedURL) {
					return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
				}
				query := req.URL.Query()
				fromImage := query.Get("fromImage")
				if fromImage != pullCase.expectedImage {
					return nil, fmt.Errorf("fromimage not set in URL query properly. Expected '%s', got %s", pullCase.expectedImage, fromImage)
				}
				tag := query.Get("tag")
				if tag != pullCase.expectedTag {
					return nil, fmt.Errorf("tag not set in URL query properly. Expected '%s', got %s", pullCase.expectedTag, tag)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte(expectedOutput))),
				}, nil
			}),
		}
		resp, err := client.ImagePull(context.Background(), pullCase.reference, image.PullOptions{
			All: pullCase.all,
		})
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != expectedOutput {
			t.Fatalf("expected '%s', got %s", expectedOutput, string(body))
		}
	}
}
