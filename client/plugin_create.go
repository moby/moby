package client // import "github.com/moby/moby/client"

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/moby/moby/api/types"
)

// PluginCreate creates a plugin
func (cli *Client) PluginCreate(ctx context.Context, createContext io.Reader, createOptions types.PluginCreateOptions) error {
	headers := http.Header(make(map[string][]string))
	headers.Set("Content-Type", "application/x-tar")

	query := url.Values{}
	query.Set("name", createOptions.RepoName)

	resp, err := cli.postRaw(ctx, "/plugins/create", query, createContext, headers)
	ensureReaderClosed(resp)
	return err
}
