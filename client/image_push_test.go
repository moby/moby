package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
)

func TestImagePushReferenceError(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, nil
		}),
	}
	// An empty reference is an invalid reference
	_, err := client.ImagePush(context.Background(), "", types.ImagePushOptions{})
	if err == nil || !strings.Contains(err.Error(), "invalid reference format") {
		t.Fatalf("expected an error, got %v", err)
	}
	// An canonical reference cannot be pushed
	_, err = client.ImagePush(context.Background(), "repo@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", types.ImagePushOptions{})
	if err == nil || err.Error() != "cannot push a digest reference" {
		t.Fatalf("expected an error, got %v", err)
	}
}

func TestImagePushAnyError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ImagePush(context.Background(), "myimage", types.ImagePushOptions{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestImagePushStatusUnauthorizedError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")),
	}
	_, err := client.ImagePush(context.Background(), "myimage", types.ImagePushOptions{})
	if !errdefs.IsUnauthorized(err) {
		t.Fatalf("expected a Unauthorized Error, got %[1]T: %[1]v", err)
	}
}

func TestImagePushWithUnauthorizedErrorAndPrivilegeFuncError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")),
	}
	privilegeFunc := func() (string, error) {
		return "", fmt.Errorf("Error requesting privilege")
	}
	_, err := client.ImagePush(context.Background(), "myimage", types.ImagePushOptions{
		PrivilegeFunc: privilegeFunc,
	})
	if err == nil || err.Error() != "Error requesting privilege" {
		t.Fatalf("expected an error requesting privilege, got %v", err)
	}
}

func TestImagePushWithUnauthorizedErrorAndAnotherUnauthorizedError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")),
	}
	privilegeFunc := func() (string, error) {
		return "a-auth-header", nil
	}
	_, err := client.ImagePush(context.Background(), "myimage", types.ImagePushOptions{
		PrivilegeFunc: privilegeFunc,
	})
	if !errdefs.IsUnauthorized(err) {
		t.Fatalf("expected a Unauthorized Error, got %[1]T: %[1]v", err)
	}
}

func TestImagePushWithPrivilegedFuncNoError(t *testing.T) {
	expectedURL := "/images/myimage/push"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
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
				return nil, fmt.Errorf("Invalid auth header : expected %s, got %s", "IAmValid", auth)
			}
			query := req.URL.Query()
			tag := query.Get("tag")
			if tag != "tag" {
				return nil, fmt.Errorf("tag not set in URL query properly. Expected '%s', got %s", "tag", tag)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("hello world"))),
			}, nil
		}),
	}
	privilegeFunc := func() (string, error) {
		return "IAmValid", nil
	}
	resp, err := client.ImagePush(context.Background(), "myimage:tag", types.ImagePushOptions{
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

func TestImagePushWithoutErrors(t *testing.T) {
	expectedOutput := "hello world"
	expectedURLFormat := "/images/%s/push"
	testCases := []struct {
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
	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%s,all-tags=%t", tc.reference, tc.all), func(t *testing.T) {
			client := &Client{
				client: newMockClient(func(req *http.Request) (*http.Response, error) {
					expectedURL := fmt.Sprintf(expectedURLFormat, tc.expectedImage)
					if !strings.HasPrefix(req.URL.Path, expectedURL) {
						return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
					}
					query := req.URL.Query()
					tag := query.Get("tag")
					if tag != tc.expectedTag {
						return nil, fmt.Errorf("tag not set in URL query properly. Expected '%s', got %s", tc.expectedTag, tag)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader([]byte(expectedOutput))),
					}, nil
				}),
			}
			resp, err := client.ImagePush(context.Background(), tc.reference, types.ImagePushOptions{
				All: tc.all,
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
		})
	}
}
