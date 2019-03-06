package client

import (
	"context"
	"net/url"
	"strconv"

	"github.com/docker/stacks/pkg/types"
)

// StackUpdate updates an existing Stack
func (cli *Client) StackUpdate(ctx context.Context, id string, version types.Version, spec types.StackSpec, options types.StackUpdateOptions) error {

	headers := map[string][]string{
		"version": {cli.settings.Version},
	}

	if options.EncodedRegistryAuth != "" {
		headers["X-Registry-Auth"] = []string{options.EncodedRegistryAuth}
	}

	query := url.Values{}
	query.Set("version", strconv.FormatUint(version.Index, 10))

	resp, err := cli.post(ctx, "/stacks/"+id, query, spec, headers)
	ensureReaderClosed(resp)
	return wrapResponseError(err, resp, "stack", id)
}
