package docker

import (
	"encoding/json"
	"github.com/dotcloud/docker"
)

type ListContainersOptions struct {
	All    bool
	Limit  int
	Since  string
	Before string
}

func (c *Client) ListContainers(opts *ListContainersOptions) ([]docker.ApiContainer, error) {
	path := "/containers/ps?" + queryString(opts)
	body, _, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var containers []docker.ApiContainer
	err = json.Unmarshal(body, &containers)
	if err != nil {
		return nil, err
	}
	return containers, nil
}

func (c *Client) InspectContainer(id string) (*docker.Container, error) {
	path := "/containers/" + id + "/json"
	body, _, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var container docker.Container
	err = json.Unmarshal(body, &container)
	if err != nil {
		return nil, err
	}
	return &container, nil
}
