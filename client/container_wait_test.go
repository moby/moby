package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"syscall"
	"testing"
	"testing/iotest"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerWaitError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	resultC, errC := client.ContainerWait(context.Background(), "nothing", "")
	select {
	case result := <-resultC:
		t.Fatalf("expected to not get a wait result, got %d", result.StatusCode)
	case err := <-errC:
		assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
	}
}

func TestContainerWait(t *testing.T) {
	expectedURL := "/containers/container_id/wait"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			b, err := json.Marshal(container.WaitResponse{
				StatusCode: 15,
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}, nil
		}),
	}

	resultC, errC := client.ContainerWait(context.Background(), "container_id", "")
	select {
	case err := <-errC:
		t.Fatal(err)
	case result := <-resultC:
		if result.StatusCode != 15 {
			t.Fatalf("expected a status code equal to '15', got %d", result.StatusCode)
		}
	}
}

func TestContainerWaitProxyInterrupt(t *testing.T) {
	expectedURL := "/v1.30/containers/container_id/wait"
	msg := "copying response body from Docker: unexpected EOF"
	client := &Client{
		version: "1.30",
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(msg)),
			}, nil
		}),
	}

	resultC, errC := client.ContainerWait(context.Background(), "container_id", "")
	select {
	case err := <-errC:
		if !strings.Contains(err.Error(), msg) {
			t.Fatalf("Expected: %s, Actual: %s", msg, err.Error())
		}
	case result := <-resultC:
		t.Fatalf("Unexpected result: %v", result)
	}
}

func TestContainerWaitProxyInterruptLong(t *testing.T) {
	expectedURL := "/v1.30/containers/container_id/wait"
	msg := strings.Repeat("x", containerWaitErrorMsgLimit*5)
	client := &Client{
		version: "1.30",
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(msg)),
			}, nil
		}),
	}

	resultC, errC := client.ContainerWait(context.Background(), "container_id", "")
	select {
	case err := <-errC:
		// LimitReader limiting isn't exact, because of how the Readers do chunking.
		if len(err.Error()) > containerWaitErrorMsgLimit*2 {
			t.Fatalf("Expected error to be limited around %d, actual length: %d", containerWaitErrorMsgLimit, len(err.Error()))
		}
	case result := <-resultC:
		t.Fatalf("Unexpected result: %v", result)
	}
}

func TestContainerWaitErrorHandling(t *testing.T) {
	for _, test := range []struct {
		name string
		rdr  io.Reader
		exp  error
	}{
		{name: "invalid json", rdr: strings.NewReader(`{]`), exp: errors.New("{]")},
		{name: "context canceled", rdr: iotest.ErrReader(context.Canceled), exp: context.Canceled},
		{name: "context deadline exceeded", rdr: iotest.ErrReader(context.DeadlineExceeded), exp: context.DeadlineExceeded},
		{name: "connection reset", rdr: iotest.ErrReader(syscall.ECONNRESET), exp: syscall.ECONNRESET},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			client := &Client{
				version: "1.30",
				client: newMockClient(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(test.rdr),
					}, nil
				}),
			}
			resultC, errC := client.ContainerWait(ctx, "container_id", "")
			select {
			case err := <-errC:
				if err.Error() != test.exp.Error() {
					t.Fatalf("ContainerWait() errC = %v; want %v", err, test.exp)
				}
				return
			case result := <-resultC:
				t.Fatalf("expected to not get a wait result, got %d", result.StatusCode)
				return
			}
			// Unexpected - we should not reach this line
		})
	}
}

func ExampleClient_ContainerWait_withTimeout() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, _ := NewClientWithOpts(FromEnv)
	_, errC := client.ContainerWait(ctx, "container_id", "")
	if err := <-errC; err != nil {
		log.Fatal(err)
	}
}
