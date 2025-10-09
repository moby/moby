package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client/pkg/jsonmessage"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImagePullReferenceParseError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		return nil, nil
	}))
	assert.NilError(t, err)
	// An empty reference is an invalid reference
	_, err = client.ImagePull(context.Background(), "", ImagePullOptions{})
	assert.Check(t, is.ErrorContains(err, "invalid reference format"))
}

func TestImagePullAnyError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.ImagePull(context.Background(), "myimage", ImagePullOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestImagePullStatusUnauthorizedError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")))
	assert.NilError(t, err)
	_, err = client.ImagePull(context.Background(), "myimage", ImagePullOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsUnauthorized))
}

func TestImagePullWithUnauthorizedErrorAndPrivilegeFuncError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")))
	assert.NilError(t, err)
	_, err = client.ImagePull(context.Background(), "myimage", ImagePullOptions{
		PrivilegeFunc: func(_ context.Context) (string, error) {
			return "", errors.New("error requesting privilege")
		},
	})
	assert.Check(t, is.Error(err, "error requesting privilege"))
}

func TestImagePullWithUnauthorizedErrorAndAnotherUnauthorizedError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusUnauthorized, "Unauthorized error")))
	assert.NilError(t, err)
	_, err = client.ImagePull(context.Background(), "myimage", ImagePullOptions{
		PrivilegeFunc: staticAuth("a-auth-header"),
	})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsUnauthorized))
}

func TestImagePullWithPrivilegedFuncNoError(t *testing.T) {
	const expectedURL = "/images/create"
	const invalidAuth = "NotValid"
	const validAuth = "IAmValid"
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
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
		fromImage := query.Get("fromImage")
		if fromImage != "docker.io/library/myimage" {
			return nil, fmt.Errorf("fromimage not set in URL query properly. Expected '%s', got %s", "docker.io/library/myimage", fromImage)
		}
		tag := query.Get("tag")
		if tag != "latest" {
			return nil, fmt.Errorf("tag not set in URL query properly. Expected '%s', got %s", "latest", tag)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte("hello world"))),
		}, nil
	}))
	assert.NilError(t, err)
	resp, err := client.ImagePull(context.Background(), "myimage", ImagePullOptions{
		RegistryAuth:  invalidAuth,
		PrivilegeFunc: staticAuth(validAuth),
	})
	assert.NilError(t, err)
	body, err := io.ReadAll(resp)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(string(body), "hello world"))
}

func TestImagePullWithoutErrors(t *testing.T) {
	const (
		expectedURL    = "/images/create"
		expectedOutput = "hello world"
	)
	pullCases := []struct {
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
	for _, pullCase := range pullCases {
		t.Run(pullCase.reference, func(t *testing.T) {
			client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
				if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
					return nil, err
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
			}))
			assert.NilError(t, err)
			resp, err := client.ImagePull(context.Background(), pullCase.reference, ImagePullOptions{
				All: pullCase.all,
			})
			assert.NilError(t, err)
			body, err := io.ReadAll(resp)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(string(body), expectedOutput))
		})
	}
}

func TestImagePullResponse(t *testing.T) {
	r, w := io.Pipe()
	response := newImagePullResponse(r)
	ctx, cancel := context.WithCancel(context.TODO())
	messages := response.JSONMessages(ctx)
	c := make(chan jsonmessage.JSONMessage)
	go func() {
		for message, err := range messages {
			if err != nil {
				close(c)
				break
			}
			c <- message
		}
	}()

	// Check we receive message sent to json stream
	w.Write([]byte(`{"id":"test"}`))
	tiemout, _ := context.WithTimeout(context.TODO(), 100*time.Millisecond)
	select {
	case message := <-c:
		assert.Equal(t, message.ID, "test")
	case <-tiemout.Done():
		t.Fatal("expected message not received")
	}

	// Check context cancelation
	cancel()
	tiemout, _ = context.WithTimeout(context.TODO(), 100*time.Millisecond)
	select {
	case _, ok := <-c:
		assert.Check(t, !ok)
	case <-tiemout.Done():
		t.Fatal("expected message not received")
	}

	// Check Close can be ran twice without error
	assert.NilError(t, response.Close())
	assert.NilError(t, response.Close())
}
