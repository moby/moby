package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestApiNetworkGetDefaults(c *check.C) {
	// By default docker daemon creates 3 networks. check if they are present
	defaults := []string{"bridge", "host", "none"}
	for _, nn := range defaults {
		c.Assert(isNetworkAvailable(c, nn), check.Equals, true)
	}
}

func (s *DockerSuite) TestApiNetworkCreateDelete(c *check.C) {
	// Create a network
	name := "testnetwork"
	id := createNetwork(c, name, true)
	c.Assert(isNetworkAvailable(c, name), check.Equals, true)

	// POST another network with same name and CheckDuplicate must fail
	createNetwork(c, name, false)

	// delete the network and make sure it is deleted
	deleteNetwork(c, id, true)
	c.Assert(isNetworkAvailable(c, name), check.Equals, false)
}

func (s *DockerSuite) TestApiNetworkFilter(c *check.C) {
	nr := getNetworkResource(c, getNetworkIDByName(c, "bridge"))
	c.Assert(nr.Name, check.Equals, "bridge")
}

func (s *DockerSuite) TestApiNetworkInspect(c *check.C) {
	// Inspect default bridge network
	nr := getNetworkResource(c, "bridge")
	c.Assert(nr.Name, check.Equals, "bridge")

	// run a container and attach it to the default bridge network
	out, _ := dockerCmd(c, "run", "-d", "--name", "test", "busybox", "top")
	containerID := strings.TrimSpace(out)
	containerIP := findContainerIP(c, "test")

	// inspect default bridge network again and make sure the container is connected
	nr = getNetworkResource(c, nr.ID)
	c.Assert(len(nr.Containers), check.Equals, 1)
	c.Assert(nr.Containers[containerID], check.NotNil)

	ip, _, err := net.ParseCIDR(nr.Containers[containerID].IPv4Address)
	c.Assert(err, check.IsNil)
	c.Assert(ip.String(), check.Equals, containerIP)
}

func (s *DockerSuite) TestApiNetworkConnectDisconnect(c *check.C) {
	// Create test network
	name := "testnetwork"
	id := createNetwork(c, name, true)
	nr := getNetworkResource(c, id)
	c.Assert(nr.Name, check.Equals, name)
	c.Assert(nr.ID, check.Equals, id)
	c.Assert(len(nr.Containers), check.Equals, 0)

	// run a container
	out, _ := dockerCmd(c, "run", "-d", "--name", "test", "busybox", "top")
	containerID := strings.TrimSpace(out)

	// connect the container to the test network
	connectNetwork(c, nr.ID, containerID)

	// inspect the network to make sure container is connected
	nr = getNetworkResource(c, nr.ID)
	c.Assert(len(nr.Containers), check.Equals, 1)
	c.Assert(nr.Containers[containerID], check.NotNil)

	// check if container IP matches network inspect
	ip, _, err := net.ParseCIDR(nr.Containers[containerID].IPv4Address)
	c.Assert(err, check.IsNil)
	containerIP := findContainerIP(c, "test")
	c.Assert(ip.String(), check.Equals, containerIP)

	// disconnect container from the network
	disconnectNetwork(c, nr.ID, containerID)
	nr = getNetworkResource(c, nr.ID)
	c.Assert(nr.Name, check.Equals, name)
	c.Assert(len(nr.Containers), check.Equals, 0)

	// delete the network
	deleteNetwork(c, nr.ID, true)
}

func isNetworkAvailable(c *check.C, name string) bool {
	status, body, err := sockRequest("GET", "/networks", nil)
	c.Assert(status, check.Equals, http.StatusOK)
	c.Assert(err, check.IsNil)

	nJSON := []types.NetworkResource{}
	err = json.Unmarshal(body, &nJSON)
	c.Assert(err, check.IsNil)

	for _, n := range nJSON {
		if n.Name == name {
			return true
		}
	}
	return false
}

func getNetworkIDByName(c *check.C, name string) string {
	var (
		v          = url.Values{}
		filterArgs = filters.Args{}
	)
	filterArgs["name"] = []string{name}
	filterJSON, err := filters.ToParam(filterArgs)
	c.Assert(err, check.IsNil)
	v.Set("filters", filterJSON)

	status, body, err := sockRequest("GET", "/networks?"+v.Encode(), nil)
	c.Assert(status, check.Equals, http.StatusOK)
	c.Assert(err, check.IsNil)

	nJSON := []types.NetworkResource{}
	err = json.Unmarshal(body, &nJSON)
	c.Assert(err, check.IsNil)
	c.Assert(len(nJSON), check.Equals, 1)

	return nJSON[0].ID
}

func getNetworkResource(c *check.C, id string) *types.NetworkResource {
	_, obj, err := sockRequest("GET", "/networks/"+id, nil)
	c.Assert(err, check.IsNil)

	nr := types.NetworkResource{}
	err = json.Unmarshal(obj, &nr)
	c.Assert(err, check.IsNil)

	return &nr
}

func createNetwork(c *check.C, name string, shouldSucceed bool) string {
	config := types.NetworkCreate{
		Name:           name,
		Driver:         "bridge",
		CheckDuplicate: true,
	}

	status, resp, err := sockRequest("POST", "/networks/create", config)
	if !shouldSucceed {
		c.Assert(status, check.Not(check.Equals), http.StatusCreated)
		return ""
	}

	c.Assert(status, check.Equals, http.StatusCreated)
	c.Assert(err, check.IsNil)

	var nr types.NetworkCreateResponse
	err = json.Unmarshal(resp, &nr)
	c.Assert(err, check.IsNil)

	return nr.ID
}

func connectNetwork(c *check.C, nid, cid string) {
	config := types.NetworkConnect{
		Container: cid,
	}

	status, _, err := sockRequest("POST", "/networks/"+nid+"/connect", config)
	c.Assert(status, check.Equals, http.StatusOK)
	c.Assert(err, check.IsNil)
}

func disconnectNetwork(c *check.C, nid, cid string) {
	config := types.NetworkConnect{
		Container: cid,
	}

	status, _, err := sockRequest("POST", "/networks/"+nid+"/disconnect", config)
	c.Assert(status, check.Equals, http.StatusOK)
	c.Assert(err, check.IsNil)
}

func deleteNetwork(c *check.C, id string, shouldSucceed bool) {
	status, _, err := sockRequest("DELETE", "/networks/"+id, nil)
	if !shouldSucceed {
		c.Assert(status, check.Not(check.Equals), http.StatusOK)
		c.Assert(err, check.NotNil)
		return
	}
	c.Assert(status, check.Equals, http.StatusOK)
	c.Assert(err, check.IsNil)
}
