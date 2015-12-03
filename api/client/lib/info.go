package lib

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/docker/docker/api/types"
)

// Info returns information about the docker server.
func (cli *Client) Info() (types.Info, error) {
	var info types.Info
	serverResp, err := cli.GET("/info", url.Values{}, nil)
	if err != nil {
		return info, err
	}
	defer serverResp.body.Close()

	if err := json.NewDecoder(serverResp.body).Decode(&info); err != nil {
		return info, fmt.Errorf("Error reading remote info: %v", err)
	}

	return info, nil
}
