package client

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

// PluginCreateOptions hold all options to plugin create.
type PluginCreateOptions struct {
	RepoName string
}

// PluginCreate creates a plugin
func (cli *Client) PluginCreate(ctx context.Context, createContext io.Reader, createOptions PluginCreateOptions) error {
	headers := http.Header(make(map[string][]string))
	headers.Set("Content-Type", "application/x-tar")

	query := url.Values{}
	query.Set("name", createOptions.RepoName)

	resp, err := cli.postRaw(ctx, "/plugins/create", query, createContext, headers)
	defer ensureReaderClosed(resp)
	return err
}
