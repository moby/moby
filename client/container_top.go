package client // import "github.com/moby/moby/client"

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/moby/moby/api/types/container"
)

// ContainerTop shows process information from within a container.
func (cli *Client) ContainerTop(ctx context.Context, containerID string, arguments []string) (container.ContainerTopOKBody, error) {
	var response container.ContainerTopOKBody
	query := url.Values{}
	if len(arguments) > 0 {
		query.Set("ps_args", strings.Join(arguments, " "))
	}

	resp, err := cli.get(ctx, "/containers/"+containerID+"/top", query, nil)
	defer ensureReaderClosed(resp)
	if err != nil {
		return response, err
	}

	err = json.NewDecoder(resp.body).Decode(&response)
	return response, err
}
