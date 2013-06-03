package docker

import (
	"encoding/json"
	"github.com/dotcloud/docker"
)

// ListContainersOptions specify parameters to the ListContainers function.
//
// See http://goo.gl/8IMr2 for more details.
type ListContainersOptions struct {
	All    bool
	Limit  int
	Since  string
	Before string
}

// ListContainers returns a slice of containers matching the given criteria.
//
// See http://goo.gl/8IMr2 for more details.
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

// InspectContainer returns information about a container by its ID.
//
// See http://goo.gl/g5tpG for more details.
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
