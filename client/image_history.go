package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/docker/docker/api/types/image"
)

// ImageHistory returns the changes in an image in history format.
func (cli *Client) ImageHistory(ctx context.Context, imageID string, opts image.HistoryOptions) ([]image.HistoryResponseItem, error) {
	values := url.Values{}
	if opts.Platform != nil {
		if err := cli.NewVersionError(ctx, "1.48", "platform"); err != nil {
			return nil, err
		}

		p, err := json.Marshal(*opts.Platform)
		if err != nil {
			return nil, fmt.Errorf("invalid platform: %v", err)
		}
		values.Set("platform", string(p))
	}

	var history []image.HistoryResponseItem
	serverResp, err := cli.get(ctx, "/images/"+imageID+"/history", values, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return history, err
	}

	err = json.NewDecoder(serverResp.body).Decode(&history)
	return history, err
}
