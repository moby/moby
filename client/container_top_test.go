package client

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerTopError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.ContainerTop(context.Background(), "nothing", ContainerTopOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerTop(context.Background(), "", ContainerTopOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerTop(context.Background(), "    ", ContainerTopOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerTop(t *testing.T) {
	const expectedURL = "/containers/container_id/top"
	expectedProcesses := [][]string{
		{"p1", "p2"},
		{"p3"},
	}
	expectedTitles := []string{"title1", "title2"}

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
		}
		query := req.URL.Query()
		args := query.Get("ps_args")
		if args != "arg1 arg2" {
			return nil, fmt.Errorf("args not set in URL query properly. Expected 'arg1 arg2', got %v", args)
		}
		return mockJSONResponse(http.StatusOK, nil, container.TopResponse{
			Processes: [][]string{
				{"p1", "p2"},
				{"p3"},
			},
			Titles: []string{"title1", "title2"},
		})(req)
	}))
	assert.NilError(t, err)

	processList, err := client.ContainerTop(context.Background(), "container_id", ContainerTopOptions{
		Arguments: []string{"arg1", "arg2"},
	})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expectedProcesses, processList.Processes))
	assert.Check(t, is.DeepEqual(expectedTitles, processList.Titles))
}
