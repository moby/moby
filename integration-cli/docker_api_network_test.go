package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/daemon/network"
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
	config := types.NetworkCreate{
		Name:           name,
		CheckDuplicate: true,
	}
	id := createNetwork(c, config, true)
	c.Assert(isNetworkAvailable(c, name), check.Equals, true)

	// POST another network with same name and CheckDuplicate must fail
	createNetwork(c, config, false)

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
	containerIP := findContainerIP(c, "test", "bridge")

	// inspect default bridge network again and make sure the container is connected
	nr = getNetworkResource(c, nr.ID)
	c.Assert(nr.Driver, check.Equals, "bridge")
	c.Assert(nr.Scope, check.Equals, "local")
	c.Assert(nr.IPAM.Driver, check.Equals, "default")
	c.Assert(len(nr.Containers), check.Equals, 1)
	c.Assert(nr.Containers[containerID], check.NotNil)

	ip, _, err := net.ParseCIDR(nr.Containers[containerID].IPv4Address)
	c.Assert(err, check.IsNil)
	c.Assert(ip.String(), check.Equals, containerIP)

	// IPAM configuration inspect
	ipam := network.IPAM{
		Driver: "default",
		Config: []network.IPAMConfig{{Subnet: "172.28.0.0/16", IPRange: "172.28.5.0/24", Gateway: "172.28.5.254"}},
	}
	config := types.NetworkCreate{
		Name:    "br0",
		Driver:  "bridge",
		IPAM:    ipam,
		Options: map[string]string{"foo": "bar", "opts": "dopts"},
	}
	id0 := createNetwork(c, config, true)
	c.Assert(isNetworkAvailable(c, "br0"), check.Equals, true)

	nr = getNetworkResource(c, id0)
	c.Assert(len(nr.IPAM.Config), check.Equals, 1)
	c.Assert(nr.IPAM.Config[0].Subnet, check.Equals, "172.28.0.0/16")
	c.Assert(nr.IPAM.Config[0].IPRange, check.Equals, "172.28.5.0/24")
	c.Assert(nr.IPAM.Config[0].Gateway, check.Equals, "172.28.5.254")
	// delete the network and make sure it is deleted
	deleteNetwork(c, id0, true)
	c.Assert(isNetworkAvailable(c, "br0"), check.Equals, false)
}

func (s *DockerSuite) TestApiNetworkConnectDisconnect(c *check.C) {
	// Create test network
	name := "testnetwork"
	config := types.NetworkCreate{
		Name: name,
	}
	id := createNetwork(c, config, true)
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
	containerIP := findContainerIP(c, "test", "testnetwork")
	c.Assert(ip.String(), check.Equals, containerIP)

	// disconnect container from the network
	disconnectNetwork(c, nr.ID, containerID)
	nr = getNetworkResource(c, nr.ID)
	c.Assert(nr.Name, check.Equals, name)
	c.Assert(len(nr.Containers), check.Equals, 0)

	// delete the network
	deleteNetwork(c, nr.ID, true)
}

func (s *DockerSuite) TestApiNetworkIpamMultipleBridgeNetworks(c *check.C) {
	// test0 bridge network
	ipam0 := network.IPAM{
		Driver: "default",
		Config: []network.IPAMConfig{{Subnet: "192.178.0.0/16", IPRange: "192.178.128.0/17", Gateway: "192.178.138.100"}},
	}
	config0 := types.NetworkCreate{
		Name:   "test0",
		Driver: "bridge",
		IPAM:   ipam0,
	}
	id0 := createNetwork(c, config0, true)
	c.Assert(isNetworkAvailable(c, "test0"), check.Equals, true)

	ipam1 := network.IPAM{
		Driver: "default",
		Config: []network.IPAMConfig{{Subnet: "192.178.128.0/17", Gateway: "192.178.128.1"}},
	}
	// test1 bridge network overlaps with test0
	config1 := types.NetworkCreate{
		Name:   "test1",
		Driver: "bridge",
		IPAM:   ipam1,
	}
	createNetwork(c, config1, false)
	c.Assert(isNetworkAvailable(c, "test1"), check.Equals, false)

	ipam2 := network.IPAM{
		Driver: "default",
		Config: []network.IPAMConfig{{Subnet: "192.169.0.0/16", Gateway: "192.169.100.100"}},
	}
	// test2 bridge network does not overlap
	config2 := types.NetworkCreate{
		Name:   "test2",
		Driver: "bridge",
		IPAM:   ipam2,
	}
	createNetwork(c, config2, true)
	c.Assert(isNetworkAvailable(c, "test2"), check.Equals, true)

	// remove test0 and retry to create test1
	deleteNetwork(c, id0, true)
	createNetwork(c, config1, true)
	c.Assert(isNetworkAvailable(c, "test1"), check.Equals, true)

	// for networks w/o ipam specified, docker will choose proper non-overlapping subnets
	createNetwork(c, types.NetworkCreate{Name: "test3"}, true)
	c.Assert(isNetworkAvailable(c, "test3"), check.Equals, true)
	createNetwork(c, types.NetworkCreate{Name: "test4"}, true)
	c.Assert(isNetworkAvailable(c, "test4"), check.Equals, true)
	createNetwork(c, types.NetworkCreate{Name: "test5"}, true)
	c.Assert(isNetworkAvailable(c, "test5"), check.Equals, true)

	for i := 1; i < 6; i++ {
		deleteNetwork(c, fmt.Sprintf("test%d", i), true)
	}
}

func (s *DockerSuite) TestApiCreateDeletePredefinedNetworks(c *check.C) {
	createDeletePredefinedNetwork(c, "bridge")
	createDeletePredefinedNetwork(c, "none")
	createDeletePredefinedNetwork(c, "host")
}

func createDeletePredefinedNetwork(c *check.C, name string) {
	// Create pre-defined network
	config := types.NetworkCreate{
		Name:           name,
		CheckDuplicate: true,
	}
	shouldSucceed := false
	createNetwork(c, config, shouldSucceed)
	deleteNetwork(c, name, shouldSucceed)
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

func createNetwork(c *check.C, config types.NetworkCreate, shouldSucceed bool) string {
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
		return
	}
	c.Assert(status, check.Equals, http.StatusOK)
	c.Assert(err, check.IsNil)
}
