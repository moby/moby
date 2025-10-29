package client

import (
	"context"
	"net/url"
	"strings"

	"github.com/containerd/errdefs"
)

// ContainerRenameOptions represents the options for renaming a container.
type ContainerRenameOptions struct {
	NewName string
}

// ContainerRenameResult represents the result of a container rename operation.
type ContainerRenameResult struct {
	// This struct can be expanded in the future if needed
}

// ContainerRename changes the name of a given container.
func (cli *Client) ContainerRename(ctx context.Context, containerID string, options ContainerRenameOptions) (ContainerRenameResult, error) {
	containerID, err := trimID("container", containerID)
	if err != nil {
		return ContainerRenameResult{}, err
	}
	options.NewName = strings.TrimSpace(options.NewName)
	if options.NewName == "" || strings.TrimPrefix(options.NewName, "/") == "" {
		// daemons before v29.0 did not handle the canonical name ("/") well
		// let's be nice and validate it here before sending
		return ContainerRenameResult{}, errdefs.ErrInvalidArgument.WithMessage("new name cannot be blank")
	}

	query := url.Values{}
	query.Set("name", options.NewName)
	resp, err := cli.post(ctx, "/containers/"+containerID+"/rename", query, nil, nil)
	defer ensureReaderClosed(resp)
	return ContainerRenameResult{}, err
}
