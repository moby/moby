package client

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"syscall"
	"testing"
	"testing/iotest"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerWaitError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	wait := client.ContainerWait(t.Context(), "nothing", ContainerWaitOptions{})
	select {
	case result := <-wait.Result:
		t.Fatalf("expected to not get a wait result, got %d", result.StatusCode)
	case err := <-wait.Error:
		assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
	}
}

// TestContainerWaitConnectionError verifies that connection errors occurring
// during API-version negotiation are not shadowed by API-version errors.
//
// Regression test for https://github.com/docker/cli/issues/4890
func TestContainerWaitConnectionError(t *testing.T) {
	client, err := New(WithAPIVersionNegotiation(), WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	wait := client.ContainerWait(t.Context(), "nothing", ContainerWaitOptions{})
	select {
	case result := <-wait.Result:
		t.Fatalf("expected to not get a wait result, got %d", result.StatusCode)
	case err := <-wait.Error:
		assert.Check(t, is.ErrorType(err, IsErrConnectionFailed))
	}
}

func TestContainerWait(t *testing.T) {
	const expectedURL = "/containers/container_id/wait"
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		return mockJSONResponse(http.StatusOK, nil, container.WaitResponse{
			StatusCode: 15,
		})(req)
	}))
	assert.NilError(t, err)

	wait := client.ContainerWait(t.Context(), "container_id", ContainerWaitOptions{})
	select {
	case err := <-wait.Error:
		assert.NilError(t, err)
	case result := <-wait.Result:
		assert.Check(t, is.Equal(result.StatusCode, int64(15)))
	}
}

func TestContainerWaitProxyInterrupt(t *testing.T) {
	const (
		expectedURL = "/containers/container_id/wait"
		expErr      = "copying response body from Docker: unexpected EOF"
	)

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		return mockResponse(http.StatusOK, nil, expErr)(req)
	}))
	assert.NilError(t, err)

	wait := client.ContainerWait(t.Context(), "container_id", ContainerWaitOptions{})
	select {
	case err := <-wait.Error:
		assert.Check(t, is.ErrorContains(err, expErr))
	case result := <-wait.Result:
		t.Fatalf("Unexpected result: %v", result)
	}
}

func TestContainerWaitProxyInterruptLong(t *testing.T) {
	const expectedURL = "/containers/container_id/wait"
	msg := strings.Repeat("x", containerWaitErrorMsgLimit*5)
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		return mockResponse(http.StatusOK, nil, msg)(req)
	}))
	assert.NilError(t, err)

	wait := client.ContainerWait(t.Context(), "container_id", ContainerWaitOptions{})
	select {
	case err := <-wait.Error:
		// LimitReader limiting isn't exact, because of how the Readers do chunking.
		assert.Check(t, len(err.Error()) <= containerWaitErrorMsgLimit*2, "Expected error to be limited around %d, actual length: %d", containerWaitErrorMsgLimit, len(err.Error()))
	case result := <-wait.Result:
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
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(test.rdr),
				}, nil
			}))
			assert.NilError(t, err)
			wait := client.ContainerWait(ctx, "container_id", ContainerWaitOptions{})
			select {
			case err := <-wait.Error:
				assert.Check(t, is.Equal(err.Error(), test.exp.Error()))
				return
			case result := <-wait.Result:
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

	client, _ := New(FromEnv)
	wait := client.ContainerWait(ctx, "container_id", ContainerWaitOptions{})
	if err := <-wait.Error; err != nil {
		log.Fatal(err)
	}
}
