package client // import "github.com/moby/moby/client"

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/moby/moby/api/types/image"
)

// ImageHistory returns the changes in an image in history format.
func (cli *Client) ImageHistory(ctx context.Context, imageID string) ([]image.HistoryResponseItem, error) {
	var history []image.HistoryResponseItem
	serverResp, err := cli.get(ctx, "/images/"+imageID+"/history", url.Values{}, nil)
	defer ensureReaderClosed(serverResp)
	if err != nil {
		return history, err
	}

	err = json.NewDecoder(serverResp.body).Decode(&history)
	return history, err
}
