package client

import (
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerKillError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerKill(t.Context(), "nothing", ContainerKillOptions{
		Signal: "SIGKILL",
	})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerKill(t.Context(), "", ContainerKillOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerKill(t.Context(), "    ", ContainerKillOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerKill(t *testing.T) {
	const expectedURL = "/containers/container_id/kill"
	const expectedSignal = "SIG_SOMETHING"
	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
				return nil, err
			}
			signal := req.URL.Query().Get("signal")
			if signal != expectedSignal {
				return nil, fmt.Errorf("signal not set in URL query properly. Expected '%s', got %s", expectedSignal, signal)
			}
			return mockResponse(http.StatusOK, nil, "")(req)
		}),
	)
	assert.NilError(t, err)

	_, err = client.ContainerKill(t.Context(), "container_id", ContainerKillOptions{
		Signal: expectedSignal,
	})
	assert.NilError(t, err)
}
