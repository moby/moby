package client

import "context"

// ConfigRemoveOptions holds options for [Client.ConfigRemove].
type ConfigRemoveOptions struct {
	// Add future optional parameters here
}

// ConfigRemoveResult holds the result of [Client.ConfigRemove].
type ConfigRemoveResult struct {
	// Add future fields here
}

// ConfigRemove removes a config.
func (cli *Client) ConfigRemove(ctx context.Context, id string, options ConfigRemoveOptions) (ConfigRemoveResult, error) {
	id, err := trimID("config", id)
	if err != nil {
		return ConfigRemoveResult{}, err
	}
	resp, err := cli.delete(ctx, "/configs/"+id, nil, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return ConfigRemoveResult{}, err
	}
	return ConfigRemoveResult{}, nil
}
