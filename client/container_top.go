package client // import "github.com/docker/docker/client"

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/docker/docker/api/types/container"
)

// ContainerTop shows process information from within a container.
// for `docker top redisstor -C "sto redis-server"`, we can't use string.Join(arguments, " ")
func (cli *Client) ContainerTop(ctx context.Context, containerID string, arguments []string) (container.ContainerTopOKBody, error) {
	var response container.ContainerTopOKBody
	query := url.Values{}
	if len(arguments) > 0 {
		argsjson, err := json.Marshal(arguments)
		if err != nil {
			return response, err
		}
		query.Set("ps_args", string(argsjson))
	}

	resp, err := cli.get(ctx, "/containers/"+containerID+"/top", query, nil)
	if err != nil {
		return response, err
	}

	err = json.NewDecoder(resp.body).Decode(&response)
	ensureReaderClosed(resp)
	return response, err
}
