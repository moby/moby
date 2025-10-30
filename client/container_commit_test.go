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

func TestContainerCommitError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerCommit(context.Background(), "nothing", ContainerCommitOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerCommit(context.Background(), "", ContainerCommitOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerCommit(context.Background(), "    ", ContainerCommitOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerCommit(t *testing.T) {
	const (
		expectedURL            = "/commit"
		expectedContainerID    = "container_id"
		specifiedReference     = "repository_name:tag"
		expectedRepositoryName = "docker.io/library/repository_name"
		expectedTag            = "tag"
		expectedComment        = "comment"
		expectedAuthor         = "author"
	)
	expectedChanges := []string{"change1", "change2"}

	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
				return nil, err
			}
			query := req.URL.Query()
			containerID := query.Get("container")
			if containerID != expectedContainerID {
				return nil, fmt.Errorf("container id not set in URL query properly. Expected '%s', got %s", expectedContainerID, containerID)
			}
			repo := query.Get("repo")
			if repo != expectedRepositoryName {
				return nil, fmt.Errorf("container repo not set in URL query properly. Expected '%s', got %s", expectedRepositoryName, repo)
			}
			tag := query.Get("tag")
			if tag != expectedTag {
				return nil, fmt.Errorf("container tag not set in URL query properly. Expected '%s', got %s'", expectedTag, tag)
			}
			comment := query.Get("comment")
			if comment != expectedComment {
				return nil, fmt.Errorf("container comment not set in URL query properly. Expected '%s', got %s'", expectedComment, comment)
			}
			author := query.Get("author")
			if author != expectedAuthor {
				return nil, fmt.Errorf("container author not set in URL query properly. Expected '%s', got %s'", expectedAuthor, author)
			}
			pause := query.Get("pause")
			if pause != "0" {
				return nil, fmt.Errorf("container pause not set in URL query properly. Expected 'true', got %v'", pause)
			}
			changes := query["changes"]
			if len(changes) != len(expectedChanges) {
				return nil, fmt.Errorf("expected container changes size to be '%d', got %d", len(expectedChanges), len(changes))
			}
			return mockJSONResponse(http.StatusOK, nil, container.CommitResponse{
				ID: "new_container_id",
			})(req)
		}),
	)
	assert.NilError(t, err)

	r, err := client.ContainerCommit(context.Background(), expectedContainerID, ContainerCommitOptions{
		Reference: specifiedReference,
		Comment:   expectedComment,
		Author:    expectedAuthor,
		Changes:   expectedChanges,
		NoPause:   true,
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(r.ID, "new_container_id"))
}
