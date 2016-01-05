package lib

import (
	"encoding/json"

	"github.com/docker/engine-api/types"
)

// ServerVersion returns information of the docker client and server host.
func (cli *Client) ServerVersion() (types.Version, error) {
	resp, err := cli.get("/version", nil, nil)
	if err != nil {
		return types.Version{}, err
	}
	defer ensureReaderClosed(resp)

	var server types.Version
	err = json.NewDecoder(resp.body).Decode(&server)
	return server, err
}
