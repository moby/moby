package client

import (
	"encoding/json"
	"net/url"

	"github.com/docker/engine-api/types"
)

// ImageHistory returns the changes in an image in history format.
func (cli *Client) ImageHistory(imageID string) ([]types.ImageHistory, error) {
	var history []types.ImageHistory
	serverResp, err := cli.get("/images/"+imageID+"/history", url.Values{}, nil)
	if err != nil {
		return history, err
	}
	defer ensureReaderClosed(serverResp)

	if err := json.NewDecoder(serverResp.body).Decode(&history); err != nil {
		return history, err
	}
	return history, nil
}
