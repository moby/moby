package lib

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/docker/engine-api/types"
)

// Info returns information about the docker server.
func (cli *Client) Info() (types.Info, error) {
	var info types.Info
	serverResp, err := cli.get("/info", url.Values{}, nil)
	if err != nil {
		return info, err
	}
	defer ensureReaderClosed(serverResp)

	if err := json.NewDecoder(serverResp.body).Decode(&info); err != nil {
		return info, fmt.Errorf("Error reading remote info: %v", err)
	}

	return info, nil
}
