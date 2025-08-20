package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/registry"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImagePushReferenceError(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, nil
		}),
	}
	// An empty reference is an invalid reference
	_, err := client.ImagePush(context.Background(), "", ImagePushOptions{})
	assert.Check(t, is.ErrorContains(err, "invalid reference format"))
	// An canonical reference cannot be pushed
	_, err = client.ImagePush(context.Background(), "repo@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", ImagePushOptions{})
	assert.Check(t, is.Error(err, "cannot push a digest reference"))
}

func TestImagePushAnyError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ImagePush(context.Background(), "myimage", ImagePushOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestImagePushStatusUnauthorizedError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")),
	}
	_, err := client.ImagePush(context.Background(), "myimage", ImagePushOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsUnauthorized))
}

func TestImagePushWithUnauthorizedErrorAndPrivilegeFuncError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")),
	}
	privilegeFunc := func(_ context.Context) (string, error) {
		return "", errors.New("Error requesting privilege")
	}
	_, err := client.ImagePush(context.Background(), "myimage", ImagePushOptions{
		PrivilegeFunc: privilegeFunc,
	})
	assert.Check(t, is.Error(err, "Error requesting privilege"))
}

func TestImagePushWithUnauthorizedErrorAndAnotherUnauthorizedError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")),
	}
	privilegeFunc := func(_ context.Context) (string, error) {
		return "a-auth-header", nil
	}
	_, err := client.ImagePush(context.Background(), "myimage", ImagePushOptions{
		PrivilegeFunc: privilegeFunc,
	})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsUnauthorized))
}

func TestImagePushWithPrivilegedFuncNoError(t *testing.T) {
	const expectedURL = "/images/docker.io/myname/myimage/push"
	const invalidAuth = "NotValid"
	const validAuth = "IAmValid"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			auth := req.Header.Get(registry.AuthHeader)
			if auth == invalidAuth {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(bytes.NewReader([]byte("Invalid credentials"))),
				}, nil
			}
			if auth != validAuth {
				return nil, fmt.Errorf("invalid auth header: expected %s, got %s", "IAmValid", auth)
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
	resp, err := client.ImagePush(context.Background(), "myname/myimage:tag", ImagePushOptions{
		RegistryAuth:  invalidAuth,
		PrivilegeFunc: staticAuth(validAuth),
	})
	assert.NilError(t, err)
	body, err := io.ReadAll(resp)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(string(body), "hello world"))
}

func TestImagePushWithoutErrors(t *testing.T) {
	const (
		expectedURLFormat = "/images/%s/push"
		expectedOutput    = "hello world"
	)
	testCases := []struct {
		all           bool
		reference     string
		expectedImage string
		expectedTag   string
	}{
		{
			all:           false,
			reference:     "myimage",
			expectedImage: "docker.io/library/myimage",
			expectedTag:   "latest",
		},
		{
			all:           false,
			reference:     "myimage:tag",
			expectedImage: "docker.io/library/myimage",
			expectedTag:   "tag",
		},
		{
			all:           true,
			reference:     "myimage",
			expectedImage: "docker.io/library/myimage",
			expectedTag:   "",
		},
		{
			all:           true,
			reference:     "myimage:anything",
			expectedImage: "docker.io/library/myimage",
			expectedTag:   "",
		},
		{
			reference:     "myname/myimage",
			expectedImage: "docker.io/myname/myimage",
			expectedTag:   "latest",
		},
		{
			reference:     "docker.io/myname/myimage",
			expectedImage: "docker.io/myname/myimage",
			expectedTag:   "latest",
		},
		{
			reference:     "index.docker.io/myname/myimage:tag",
			expectedImage: "docker.io/myname/myimage",
			expectedTag:   "tag",
		},
		{
			reference:     "localhost/myname/myimage",
			expectedImage: "localhost/myname/myimage",
			expectedTag:   "latest",
		},
		{
			reference:     "registry.example.com:5000/myimage:tag",
			expectedImage: "registry.example.com:5000/myimage",
			expectedTag:   "tag",
		},
	}
	for _, tc := range testCases {
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
			resp, err := client.ImagePush(context.Background(), tc.reference, ImagePushOptions{
				All: tc.all,
			})
			assert.NilError(t, err)
			body, err := io.ReadAll(resp)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(string(body), expectedOutput))
		})
	}
}
