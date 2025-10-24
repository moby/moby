package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/container"
)

// ContainerCommitOptions holds parameters to commit changes into a container.
type ContainerCommitOptions struct {
	Reference string
	Comment   string
	Author    string
	Changes   []string
	NoPause   bool // NoPause disables pausing the container during commit.
	Config    *container.Config
}

// ContainerCommitResult is the result from committing a container.
type ContainerCommitResult struct {
	ID string
}

// ContainerCommit applies changes to a container and creates a new tagged image.
func (cli *Client) ContainerCommit(ctx context.Context, containerID string, options ContainerCommitOptions) (ContainerCommitResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerCommitResult{}, err
	}

	var repository, tag string
	if options.Reference != "" {
		ref, err := reference.ParseNormalizedNamed(options.Reference)
		if err != nil {
			return ContainerCommitResult{}, err
		}

		if _, ok := ref.(reference.Digested); ok {
			return ContainerCommitResult{}, errors.New("refusing to create a tag with a digest reference")
		}
		ref = reference.TagNameOnly(ref)

		if tagged, ok := ref.(reference.Tagged); ok {
			tag = tagged.Tag()
		}
		repository = ref.Name()
	}

	query := url.Values{}
	query.Set("container", containerID)
	query.Set("repo", repository)
	query.Set("tag", tag)
	query.Set("comment", options.Comment)
	query.Set("author", options.Author)
	for _, change := range options.Changes {
		query.Add("changes", change)
	}
	if options.NoPause {
		query.Set("pause", "0")
	}

	var response container.CommitResponse
	resp, err := cli.post(ctx, "/commit", query, options.Config, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ContainerCommitResult{}, err
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	return ContainerCommitResult{ID: response.ID}, err
}
