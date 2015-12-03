package lib

import (
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types"
)

// ImageHistory returns the changes in an image in history format.
func (cli *Client) ImageHistory(imageID string) ([]types.ImageHistory, error) {
	var history []types.ImageHistory
	serverResp, err := cli.GET("/images/"+imageID+"/history", url.Values{}, nil)
	if err != nil {
		return history, err
	}
	defer serverResp.body.Close()

	if err := json.NewDecoder(serverResp.body).Decode(&history); err != nil {
		return history, err
	}
	return history, nil
}
