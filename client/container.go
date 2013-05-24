package docker

import (
	"encoding/json"
	"github.com/dotcloud/docker"
	"net/http"
)

type ListContainersOptions struct {
	All    bool
	Limit  int
	Since  string
	Before string
}

func (c *Client) ListContainers(opts *ListContainersOptions) ([]docker.ApiContainer, error) {
	url := c.getURL("/containers/ps") + "?" + queryString(opts)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, newApiClientError(resp)
	}
	var containers []docker.ApiContainer
	err = json.NewDecoder(resp.Body).Decode(&containers)
	if err != nil {
		return nil, err
	}
	return containers, nil
}
