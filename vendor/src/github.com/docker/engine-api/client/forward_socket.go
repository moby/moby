package client

import (
	"net/url"

	"github.com/docker/engine-api/types"
)

func (cli *Client) ContainerForwardSocket(id string) (types.HijackedResponse, error) {
	headers := map[string][]string{"Content-Type": {"application/son"}}

	body := types.ContainerForwardSocketConfig{}
	return cli.postHijacked("/containers/"+id+"/forwardSocket", url.Values{}, body, headers)
}
