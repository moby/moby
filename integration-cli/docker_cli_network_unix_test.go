// +build !windows

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions/v1p20"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/docker/runconfig"
	"github.com/docker/libnetwork/driverapi"
	remoteapi "github.com/docker/libnetwork/drivers/remote/api"
	"github.com/docker/libnetwork/ipamapi"
	remoteipam "github.com/docker/libnetwork/ipams/remote/api"
	"github.com/docker/libnetwork/netlabel"
	"github.com/go-check/check"
	"github.com/vishvananda/netlink"
)

const dummyNetworkDriver = "dummy-network-driver"
const dummyIpamDriver = "dummy-ipam-driver"

var remoteDriverNetworkRequest remoteapi.CreateNetworkRequest

func init() {
	check.Suite(&DockerNetworkSuite{
		ds: &DockerSuite{},
	})
}

type DockerNetworkSuite struct {
	server *httptest.Server
	ds     *DockerSuite
	d      *Daemon
}

func (s *DockerNetworkSuite) SetUpTest(c *check.C) {
	s.d = NewDaemon(c)
}

func (s *DockerNetworkSuite) TearDownTest(c *check.C) {
	s.d.Stop()
	s.ds.TearDownTest(c)
}

func (s *DockerNetworkSuite) SetUpSuite(c *check.C) {
	mux := http.NewServeMux()
	s.server = httptest.NewServer(mux)
	c.Assert(s.server, check.NotNil, check.Commentf("Failed to start a HTTP Server"))

	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintf(w, `{"Implements": ["%s", "%s"]}`, driverapi.NetworkPluginEndpointType, ipamapi.PluginEndpointType)
	})

	// Network driver implementation
	mux.HandleFunc(fmt.Sprintf("/%s.GetCapabilities", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintf(w, `{"Scope":"local"}`)
	})

	mux.HandleFunc(fmt.Sprintf("/%s.CreateNetwork", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&remoteDriverNetworkRequest)
		if err != nil {
			http.Error(w, "Unable to decode JSON payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintf(w, "null")
	})

	mux.HandleFunc(fmt.Sprintf("/%s.DeleteNetwork", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintf(w, "null")
	})

	mux.HandleFunc(fmt.Sprintf("/%s.CreateEndpoint", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintf(w, `{"Interface":{"MacAddress":"a0:b1:c2:d3:e4:f5"}}`)
	})

	mux.HandleFunc(fmt.Sprintf("/%s.Join", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")

		veth := &netlink.Veth{
			LinkAttrs: netlink.LinkAttrs{Name: "randomIfName", TxQLen: 0}, PeerName: "cnt0"}
		if err := netlink.LinkAdd(veth); err != nil {
			fmt.Fprintf(w, `{"Error":"failed to add veth pair: `+err.Error()+`"}`)
		} else {
			fmt.Fprintf(w, `{"InterfaceName":{ "SrcName":"cnt0", "DstPrefix":"veth"}}`)
		}
	})

	mux.HandleFunc(fmt.Sprintf("/%s.Leave", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintf(w, "null")
	})

	mux.HandleFunc(fmt.Sprintf("/%s.DeleteEndpoint", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		if link, err := netlink.LinkByName("cnt0"); err == nil {
			netlink.LinkDel(link)
		}
		fmt.Fprintf(w, "null")
	})

	// Ipam Driver implementation
	var (
		poolRequest       remoteipam.RequestPoolRequest
		poolReleaseReq    remoteipam.ReleasePoolRequest
		addressRequest    remoteipam.RequestAddressRequest
		addressReleaseReq remoteipam.ReleaseAddressRequest
		lAS               = "localAS"
		gAS               = "globalAS"
		pool              = "172.28.0.0/16"
		poolID            = lAS + "/" + pool
		gw                = "172.28.255.254/16"
	)

	mux.HandleFunc(fmt.Sprintf("/%s.GetDefaultAddressSpaces", ipamapi.PluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintf(w, `{"LocalDefaultAddressSpace":"`+lAS+`", "GlobalDefaultAddressSpace": "`+gAS+`"}`)
	})

	mux.HandleFunc(fmt.Sprintf("/%s.RequestPool", ipamapi.PluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&poolRequest)
		if err != nil {
			http.Error(w, "Unable to decode JSON payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		if poolRequest.AddressSpace != lAS && poolRequest.AddressSpace != gAS {
			fmt.Fprintf(w, `{"Error":"Unknown address space in pool request: `+poolRequest.AddressSpace+`"}`)
		} else if poolRequest.Pool != "" && poolRequest.Pool != pool {
			fmt.Fprintf(w, `{"Error":"Cannot handle explicit pool requests yet"}`)
		} else {
			fmt.Fprintf(w, `{"PoolID":"`+poolID+`", "Pool":"`+pool+`"}`)
		}
	})

	mux.HandleFunc(fmt.Sprintf("/%s.RequestAddress", ipamapi.PluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&addressRequest)
		if err != nil {
			http.Error(w, "Unable to decode JSON payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		// make sure libnetwork is now querying on the expected pool id
		if addressRequest.PoolID != poolID {
			fmt.Fprintf(w, `{"Error":"unknown pool id"}`)
		} else if addressRequest.Address != "" {
			fmt.Fprintf(w, `{"Error":"Cannot handle explicit address requests yet"}`)
		} else {
			fmt.Fprintf(w, `{"Address":"`+gw+`"}`)
		}
	})

	mux.HandleFunc(fmt.Sprintf("/%s.ReleaseAddress", ipamapi.PluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&addressReleaseReq)
		if err != nil {
			http.Error(w, "Unable to decode JSON payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		// make sure libnetwork is now asking to release the expected address fro mthe expected poolid
		if addressRequest.PoolID != poolID {
			fmt.Fprintf(w, `{"Error":"unknown pool id"}`)
		} else if addressReleaseReq.Address != gw {
			fmt.Fprintf(w, `{"Error":"unknown address"}`)
		} else {
			fmt.Fprintf(w, "null")
		}
	})

	mux.HandleFunc(fmt.Sprintf("/%s.ReleasePool", ipamapi.PluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&poolReleaseReq)
		if err != nil {
			http.Error(w, "Unable to decode JSON payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		// make sure libnetwork is now asking to release the expected poolid
		if addressRequest.PoolID != poolID {
			fmt.Fprintf(w, `{"Error":"unknown pool id"}`)
		} else {
			fmt.Fprintf(w, "null")
		}
	})

	err := os.MkdirAll("/etc/docker/plugins", 0755)
	c.Assert(err, checker.IsNil)

	fileName := fmt.Sprintf("/etc/docker/plugins/%s.spec", dummyNetworkDriver)
	err = ioutil.WriteFile(fileName, []byte(s.server.URL), 0644)
	c.Assert(err, checker.IsNil)

	ipamFileName := fmt.Sprintf("/etc/docker/plugins/%s.spec", dummyIpamDriver)
	err = ioutil.WriteFile(ipamFileName, []byte(s.server.URL), 0644)
	c.Assert(err, checker.IsNil)
}

func (s *DockerNetworkSuite) TearDownSuite(c *check.C) {
	if s.server == nil {
		return
	}

	s.server.Close()

	err := os.RemoveAll("/etc/docker/plugins")
	c.Assert(err, checker.IsNil)
}

func assertNwIsAvailable(c *check.C, name string) {
	if !isNwPresent(c, name) {
		c.Fatalf("Network %s not found in network ls o/p", name)
	}
}

func assertNwNotAvailable(c *check.C, name string) {
	if isNwPresent(c, name) {
		c.Fatalf("Found network %s in network ls o/p", name)
	}
}

func isNwPresent(c *check.C, name string) bool {
	out, _ := dockerCmd(c, "network", "ls")
	lines := strings.Split(out, "\n")
	for i := 1; i < len(lines)-1; i++ {
		netFields := strings.Fields(lines[i])
		if netFields[1] == name {
			return true
		}
	}
	return false
}

func getNwResource(c *check.C, name string) *types.NetworkResource {
	out, _ := dockerCmd(c, "network", "inspect", name)
	nr := []types.NetworkResource{}
	err := json.Unmarshal([]byte(out), &nr)
	c.Assert(err, check.IsNil)
	return &nr[0]
}

func (s *DockerNetworkSuite) TestDockerNetworkLsDefault(c *check.C) {
	defaults := []string{"bridge", "host", "none"}
	for _, nn := range defaults {
		assertNwIsAvailable(c, nn)
	}
}

func (s *DockerNetworkSuite) TestDockerNetworkCreateDelete(c *check.C) {
	dockerCmd(c, "network", "create", "test")
	assertNwIsAvailable(c, "test")

	dockerCmd(c, "network", "rm", "test")
	assertNwNotAvailable(c, "test")
}

func (s *DockerSuite) TestDockerNetworkDeleteNotExists(c *check.C) {
	out, _, err := dockerCmdWithError("network", "rm", "test")
	c.Assert(err, checker.NotNil, check.Commentf("%v", out))
}

func (s *DockerSuite) TestDockerNetworkDeleteMultiple(c *check.C) {
	dockerCmd(c, "network", "create", "testDelMulti0")
	assertNwIsAvailable(c, "testDelMulti0")
	dockerCmd(c, "network", "create", "testDelMulti1")
	assertNwIsAvailable(c, "testDelMulti1")
	dockerCmd(c, "network", "create", "testDelMulti2")
	assertNwIsAvailable(c, "testDelMulti2")
	out, _ := dockerCmd(c, "run", "-d", "--net", "testDelMulti2", "busybox", "top")
	waitRun(strings.TrimSpace(out))

	// delete three networks at the same time, since testDelMulti2
	// contains active container, it's deletion should fail.
	out, _, err := dockerCmdWithError("network", "rm", "testDelMulti0", "testDelMulti1", "testDelMulti2")
	// err should not be nil due to deleting testDelMulti2 failed.
	c.Assert(err, checker.NotNil, check.Commentf("out: %s", out))
	// testDelMulti2 should fail due to network has active endpoints
	c.Assert(out, checker.Contains, "has active endpoints")
	assertNwNotAvailable(c, "testDelMulti0")
	assertNwNotAvailable(c, "testDelMulti1")
	// testDelMulti2 can't be deleted, so it should exists
	assertNwIsAvailable(c, "testDelMulti2")
}

func (s *DockerSuite) TestDockerInspectMultipleNetwork(c *check.C) {
	out, _ := dockerCmd(c, "network", "inspect", "host", "none")
	networkResources := []types.NetworkResource{}
	err := json.Unmarshal([]byte(out), &networkResources)
	c.Assert(err, check.IsNil)
	c.Assert(networkResources, checker.HasLen, 2)

	// Should print an error, return an exitCode 1 *but* should print the host network
	out, exitCode, err := dockerCmdWithError("network", "inspect", "host", "nonexistent")
	c.Assert(err, checker.NotNil)
	c.Assert(exitCode, checker.Equals, 1)
	c.Assert(out, checker.Contains, "Error: No such network: nonexistent")
	networkResources = []types.NetworkResource{}
	inspectOut := strings.SplitN(out, "\n", 2)[1]
	err = json.Unmarshal([]byte(inspectOut), &networkResources)
	c.Assert(networkResources, checker.HasLen, 1)

	// Should print an error and return an exitCode, nothing else
	out, exitCode, err = dockerCmdWithError("network", "inspect", "nonexistent")
	c.Assert(err, checker.NotNil)
	c.Assert(exitCode, checker.Equals, 1)
	c.Assert(out, checker.Contains, "Error: No such network: nonexistent")
}

func (s *DockerSuite) TestDockerInspectNetworkWithContainerName(c *check.C) {
	dockerCmd(c, "network", "create", "brNetForInspect")
	assertNwIsAvailable(c, "brNetForInspect")
	defer func() {
		dockerCmd(c, "network", "rm", "brNetForInspect")
		assertNwNotAvailable(c, "brNetForInspect")
	}()

	out, _ := dockerCmd(c, "run", "-d", "--name", "testNetInspect1", "--net", "brNetForInspect", "busybox", "top")
	c.Assert(waitRun("testNetInspect1"), check.IsNil)
	containerID := strings.TrimSpace(out)
	defer func() {
		// we don't stop container by name, because we'll rename it later
		dockerCmd(c, "stop", containerID)
	}()

	out, _ = dockerCmd(c, "network", "inspect", "brNetForInspect")
	networkResources := []types.NetworkResource{}
	err := json.Unmarshal([]byte(out), &networkResources)
	c.Assert(err, check.IsNil)
	c.Assert(networkResources, checker.HasLen, 1)
	container, ok := networkResources[0].Containers[containerID]
	c.Assert(ok, checker.True)
	c.Assert(container.Name, checker.Equals, "testNetInspect1")

	// rename container and check docker inspect output update
	newName := "HappyNewName"
	dockerCmd(c, "rename", "testNetInspect1", newName)

	// check whether network inspect works properly
	out, _ = dockerCmd(c, "network", "inspect", "brNetForInspect")
	newNetRes := []types.NetworkResource{}
	err = json.Unmarshal([]byte(out), &newNetRes)
	c.Assert(err, check.IsNil)
	c.Assert(newNetRes, checker.HasLen, 1)
	container1, ok := newNetRes[0].Containers[containerID]
	c.Assert(ok, checker.True)
	c.Assert(container1.Name, checker.Equals, newName)

}

func (s *DockerNetworkSuite) TestDockerNetworkConnectDisconnect(c *check.C) {
	dockerCmd(c, "network", "create", "test")
	assertNwIsAvailable(c, "test")
	nr := getNwResource(c, "test")

	c.Assert(nr.Name, checker.Equals, "test")
	c.Assert(len(nr.Containers), checker.Equals, 0)

	// run a container
	out, _ := dockerCmd(c, "run", "-d", "--name", "test", "busybox", "top")
	c.Assert(waitRun("test"), check.IsNil)
	containerID := strings.TrimSpace(out)

	// connect the container to the test network
	dockerCmd(c, "network", "connect", "test", containerID)

	// inspect the network to make sure container is connected
	nr = getNetworkResource(c, nr.ID)
	c.Assert(len(nr.Containers), checker.Equals, 1)
	c.Assert(nr.Containers[containerID], check.NotNil)

	// check if container IP matches network inspect
	ip, _, err := net.ParseCIDR(nr.Containers[containerID].IPv4Address)
	c.Assert(err, check.IsNil)
	containerIP := findContainerIP(c, "test", "test")
	c.Assert(ip.String(), checker.Equals, containerIP)

	// disconnect container from the network
	dockerCmd(c, "network", "disconnect", "test", containerID)
	nr = getNwResource(c, "test")
	c.Assert(nr.Name, checker.Equals, "test")
	c.Assert(len(nr.Containers), checker.Equals, 0)

	// check if network connect fails for inactive containers
	dockerCmd(c, "stop", containerID)
	_, _, err = dockerCmdWithError("network", "connect", "test", containerID)
	c.Assert(err, check.NotNil)

	dockerCmd(c, "network", "rm", "test")
	assertNwNotAvailable(c, "test")
}

func (s *DockerNetworkSuite) TestDockerNetworkIpamMultipleNetworks(c *check.C) {
	// test0 bridge network
	dockerCmd(c, "network", "create", "--subnet=192.168.0.0/16", "test1")
	assertNwIsAvailable(c, "test1")

	// test2 bridge network does not overlap
	dockerCmd(c, "network", "create", "--subnet=192.169.0.0/16", "test2")
	assertNwIsAvailable(c, "test2")

	// for networks w/o ipam specified, docker will choose proper non-overlapping subnets
	dockerCmd(c, "network", "create", "test3")
	assertNwIsAvailable(c, "test3")
	dockerCmd(c, "network", "create", "test4")
	assertNwIsAvailable(c, "test4")
	dockerCmd(c, "network", "create", "test5")
	assertNwIsAvailable(c, "test5")

	// test network with multiple subnets
	// bridge network doesnt support multiple subnets. hence, use a dummy driver that supports

	dockerCmd(c, "network", "create", "-d", dummyNetworkDriver, "--subnet=192.168.0.0/16", "--subnet=192.170.0.0/16", "test6")
	assertNwIsAvailable(c, "test6")

	// test network with multiple subnets with valid ipam combinations
	// also check same subnet across networks when the driver supports it.
	dockerCmd(c, "network", "create", "-d", dummyNetworkDriver,
		"--subnet=192.168.0.0/16", "--subnet=192.170.0.0/16",
		"--gateway=192.168.0.100", "--gateway=192.170.0.100",
		"--ip-range=192.168.1.0/24",
		"--aux-address", "a=192.168.1.5", "--aux-address", "b=192.168.1.6",
		"--aux-address", "a=192.170.1.5", "--aux-address", "b=192.170.1.6",
		"test7")
	assertNwIsAvailable(c, "test7")

	// cleanup
	for i := 1; i < 8; i++ {
		dockerCmd(c, "network", "rm", fmt.Sprintf("test%d", i))
	}
}

func (s *DockerNetworkSuite) TestDockerNetworkCustomIpam(c *check.C) {
	// Create a bridge network using custom ipam driver
	dockerCmd(c, "network", "create", "--ipam-driver", dummyIpamDriver, "br0")
	assertNwIsAvailable(c, "br0")

	// Verify expected network ipam fields are there
	nr := getNetworkResource(c, "br0")
	c.Assert(nr.Driver, checker.Equals, "bridge")
	c.Assert(nr.IPAM.Driver, checker.Equals, dummyIpamDriver)

	// remove network and exercise remote ipam driver
	dockerCmd(c, "network", "rm", "br0")
	assertNwNotAvailable(c, "br0")
}

func (s *DockerNetworkSuite) TestDockerNetworkInspect(c *check.C) {
	// if unspecified, network gateway will be selected from inside preferred pool
	dockerCmd(c, "network", "create", "--driver=bridge", "--subnet=172.28.0.0/16", "--ip-range=172.28.5.0/24", "--gateway=172.28.5.254", "br0")
	assertNwIsAvailable(c, "br0")

	nr := getNetworkResource(c, "br0")
	c.Assert(nr.Driver, checker.Equals, "bridge")
	c.Assert(nr.Scope, checker.Equals, "local")
	c.Assert(nr.IPAM.Driver, checker.Equals, "default")
	c.Assert(len(nr.IPAM.Config), checker.Equals, 1)
	c.Assert(nr.IPAM.Config[0].Subnet, checker.Equals, "172.28.0.0/16")
	c.Assert(nr.IPAM.Config[0].IPRange, checker.Equals, "172.28.5.0/24")
	c.Assert(nr.IPAM.Config[0].Gateway, checker.Equals, "172.28.5.254")
	dockerCmd(c, "network", "rm", "br0")
}

func (s *DockerNetworkSuite) TestDockerNetworkIpamInvalidCombinations(c *check.C) {
	// network with ip-range out of subnet range
	_, _, err := dockerCmdWithError("network", "create", "--subnet=192.168.0.0/16", "--ip-range=192.170.0.0/16", "test")
	c.Assert(err, check.NotNil)

	// network with multiple gateways for a single subnet
	_, _, err = dockerCmdWithError("network", "create", "--subnet=192.168.0.0/16", "--gateway=192.168.0.1", "--gateway=192.168.0.2", "test")
	c.Assert(err, check.NotNil)

	// Multiple overlaping subnets in the same network must fail
	_, _, err = dockerCmdWithError("network", "create", "--subnet=192.168.0.0/16", "--subnet=192.168.1.0/16", "test")
	c.Assert(err, check.NotNil)

	// overlapping subnets across networks must fail
	// create a valid test0 network
	dockerCmd(c, "network", "create", "--subnet=192.168.0.0/16", "test0")
	assertNwIsAvailable(c, "test0")
	// create an overlapping test1 network
	_, _, err = dockerCmdWithError("network", "create", "--subnet=192.168.128.0/17", "test1")
	c.Assert(err, check.NotNil)
	dockerCmd(c, "network", "rm", "test0")
}

func (s *DockerNetworkSuite) TestDockerNetworkDriverOptions(c *check.C) {
	dockerCmd(c, "network", "create", "-d", dummyNetworkDriver, "-o", "opt1=drv1", "-o", "opt2=drv2", "testopt")
	assertNwIsAvailable(c, "testopt")
	gopts := remoteDriverNetworkRequest.Options[netlabel.GenericData]
	c.Assert(gopts, checker.NotNil)
	opts, ok := gopts.(map[string]interface{})
	c.Assert(ok, checker.Equals, true)
	c.Assert(opts["opt1"], checker.Equals, "drv1")
	c.Assert(opts["opt2"], checker.Equals, "drv2")
	dockerCmd(c, "network", "rm", "testopt")

}

func (s *DockerDaemonSuite) TestDockerNetworkNoDiscoveryDefaultBridgeNetwork(c *check.C) {
	testRequires(c, ExecSupport)
	// On default bridge network built-in service discovery should not happen
	hostsFile := "/etc/hosts"
	bridgeName := "external-bridge"
	bridgeIP := "192.169.255.254/24"
	out, err := createInterface(c, "bridge", bridgeName, bridgeIP)
	c.Assert(err, check.IsNil, check.Commentf(out))
	defer deleteInterface(c, bridgeName)

	err = s.d.StartWithBusybox("--bridge", bridgeName)
	c.Assert(err, check.IsNil)
	defer s.d.Restart()

	// run two containers and store first container's etc/hosts content
	out, err = s.d.Cmd("run", "-d", "busybox", "top")
	c.Assert(err, check.IsNil)
	cid1 := strings.TrimSpace(out)
	defer s.d.Cmd("stop", cid1)

	hosts, err := s.d.Cmd("exec", cid1, "cat", hostsFile)
	c.Assert(err, checker.IsNil)

	out, err = s.d.Cmd("run", "-d", "--name", "container2", "busybox", "top")
	c.Assert(err, check.IsNil)
	cid2 := strings.TrimSpace(out)

	// verify first container's etc/hosts file has not changed after spawning the second named container
	hostsPost, err := s.d.Cmd("exec", cid1, "cat", hostsFile)
	c.Assert(err, checker.IsNil)
	c.Assert(string(hosts), checker.Equals, string(hostsPost),
		check.Commentf("Unexpected %s change on second container creation", hostsFile))

	// stop container 2 and verify first container's etc/hosts has not changed
	_, err = s.d.Cmd("stop", cid2)
	c.Assert(err, check.IsNil)

	hostsPost, err = s.d.Cmd("exec", cid1, "cat", hostsFile)
	c.Assert(err, checker.IsNil)
	c.Assert(string(hosts), checker.Equals, string(hostsPost),
		check.Commentf("Unexpected %s change on second container creation", hostsFile))

	// but discovery is on when connecting to non default bridge network
	network := "anotherbridge"
	out, err = s.d.Cmd("network", "create", network)
	c.Assert(err, check.IsNil, check.Commentf(out))
	defer s.d.Cmd("network", "rm", network)

	out, err = s.d.Cmd("network", "connect", network, cid1)
	c.Assert(err, check.IsNil, check.Commentf(out))

	hostsPost, err = s.d.Cmd("exec", cid1, "cat", hostsFile)
	c.Assert(err, checker.IsNil)
	c.Assert(string(hosts), checker.Equals, string(hostsPost),
		check.Commentf("Unexpected %s change on second network connection", hostsFile))

	cName := "container3"
	out, err = s.d.Cmd("run", "-d", "--net", network, "--name", cName, "busybox", "top")
	c.Assert(err, check.IsNil, check.Commentf(out))
	cid3 := strings.TrimSpace(out)
	defer s.d.Cmd("stop", cid3)

	// container1 etc/hosts file should contain an entry for the third container
	hostsPost, err = s.d.Cmd("exec", cid1, "cat", hostsFile)
	c.Assert(err, checker.IsNil)
	c.Assert(string(hostsPost), checker.Contains, cName,
		check.Commentf("Container 1  %s file does not contain entries for named container %q: %s", hostsFile, cName, string(hostsPost)))

	// on container3 disconnect, first container's etc/hosts should go back to original form
	out, err = s.d.Cmd("network", "disconnect", network, cid3)
	c.Assert(err, check.IsNil, check.Commentf(out))

	hostsPost, err = s.d.Cmd("exec", cid1, "cat", hostsFile)
	c.Assert(err, checker.IsNil)
	c.Assert(string(hosts), checker.Equals, string(hostsPost),
		check.Commentf("Unexpected %s content after disconnecting from second network", hostsFile))
}

func (s *DockerNetworkSuite) TestDockerNetworkAnonymousEndpoint(c *check.C) {
	testRequires(c, ExecSupport)
	hostsFile := "/etc/hosts"
	cstmBridgeNw := "custom-bridge-nw"
	cstmBridgeNw1 := "custom-bridge-nw1"

	dockerCmd(c, "network", "create", "-d", "bridge", cstmBridgeNw)
	assertNwIsAvailable(c, cstmBridgeNw)

	// run two anonymous containers and store their etc/hosts content
	out, _ := dockerCmd(c, "run", "-d", "--net", cstmBridgeNw, "busybox", "top")
	cid1 := strings.TrimSpace(out)

	hosts1, err := readContainerFileWithExec(cid1, hostsFile)
	c.Assert(err, checker.IsNil)

	out, _ = dockerCmd(c, "run", "-d", "--net", cstmBridgeNw, "busybox", "top")
	cid2 := strings.TrimSpace(out)

	hosts2, err := readContainerFileWithExec(cid2, hostsFile)
	c.Assert(err, checker.IsNil)

	// verify first container etc/hosts file has not changed
	hosts1post, err := readContainerFileWithExec(cid1, hostsFile)
	c.Assert(err, checker.IsNil)
	c.Assert(string(hosts1), checker.Equals, string(hosts1post),
		check.Commentf("Unexpected %s change on anonymous container creation", hostsFile))

	// Connect the 2nd container to a new network and verify the
	// first container /etc/hosts file still hasn't changed.
	dockerCmd(c, "network", "create", "-d", "bridge", cstmBridgeNw1)
	assertNwIsAvailable(c, cstmBridgeNw1)

	dockerCmd(c, "network", "connect", cstmBridgeNw1, cid2)

	hosts1post, err = readContainerFileWithExec(cid1, hostsFile)
	c.Assert(err, checker.IsNil)
	c.Assert(string(hosts1), checker.Equals, string(hosts1post),
		check.Commentf("Unexpected %s change on container connect", hostsFile))

	// start a named container
	cName := "AnyName"
	out, _ = dockerCmd(c, "run", "-d", "--net", cstmBridgeNw, "--name", cName, "busybox", "top")
	cid3 := strings.TrimSpace(out)

	// verify etc/hosts file for first two containers contains the named container entry
	hosts1post, err = readContainerFileWithExec(cid1, hostsFile)
	c.Assert(err, checker.IsNil)
	c.Assert(string(hosts1post), checker.Contains, cName,
		check.Commentf("Container 1  %s file does not contain entries for named container %q: %s", hostsFile, cName, string(hosts1post)))

	hosts2post, err := readContainerFileWithExec(cid2, hostsFile)
	c.Assert(err, checker.IsNil)
	c.Assert(string(hosts2post), checker.Contains, cName,
		check.Commentf("Container 2  %s file does not contain entries for named container %q: %s", hostsFile, cName, string(hosts2post)))

	// Stop named container and verify first two containers' etc/hosts entries are back to original
	dockerCmd(c, "stop", cid3)
	hosts1post, err = readContainerFileWithExec(cid1, hostsFile)
	c.Assert(err, checker.IsNil)
	c.Assert(string(hosts1), checker.Equals, string(hosts1post),
		check.Commentf("Unexpected %s change on anonymous container creation", hostsFile))

	hosts2post, err = readContainerFileWithExec(cid2, hostsFile)
	c.Assert(err, checker.IsNil)
	c.Assert(string(hosts2), checker.Equals, string(hosts2post),
		check.Commentf("Unexpected %s change on anonymous container creation", hostsFile))
}

func (s *DockerNetworkSuite) TestDockerNetworkLinkOndefaultNetworkOnly(c *check.C) {
	// Link feature must work only on default network, and not across networks
	cnt1 := "container1"
	cnt2 := "container2"
	network := "anotherbridge"

	// Run first container on default network
	dockerCmd(c, "run", "-d", "--name", cnt1, "busybox", "top")

	// Create another network and run the second container on it
	dockerCmd(c, "network", "create", network)
	assertNwIsAvailable(c, network)
	dockerCmd(c, "run", "-d", "--net", network, "--name", cnt2, "busybox", "top")

	// Try launching a container on default network, linking to the first container. Must succeed
	dockerCmd(c, "run", "-d", "--link", fmt.Sprintf("%s:%s", cnt1, cnt1), "busybox", "top")

	// Try launching a container on default network, linking to the second container. Must fail
	_, _, err := dockerCmdWithError("run", "-d", "--link", fmt.Sprintf("%s:%s", cnt2, cnt2), "busybox", "top")
	c.Assert(err, checker.NotNil)

	// Connect second container to default network. Now a container on default network can link to it
	dockerCmd(c, "network", "connect", "bridge", cnt2)
	dockerCmd(c, "run", "-d", "--link", fmt.Sprintf("%s:%s", cnt2, cnt2), "busybox", "top")
}

func (s *DockerNetworkSuite) TestDockerNetworkOverlayPortMapping(c *check.C) {
	// Verify exposed ports are present in ps output when running a container on
	// a network managed by a driver which does not provide the default gateway
	// for the container
	nwn := "ov"
	ctn := "bb"
	port1 := 80
	port2 := 443
	expose1 := fmt.Sprintf("--expose=%d", port1)
	expose2 := fmt.Sprintf("--expose=%d", port2)

	dockerCmd(c, "network", "create", "-d", dummyNetworkDriver, nwn)
	assertNwIsAvailable(c, nwn)

	dockerCmd(c, "run", "-d", "--net", nwn, "--name", ctn, expose1, expose2, "busybox", "top")

	// Check docker ps o/p for last created container reports the unpublished ports
	unpPort1 := fmt.Sprintf("%d/tcp", port1)
	unpPort2 := fmt.Sprintf("%d/tcp", port2)
	out, _ := dockerCmd(c, "ps", "-n=1")
	// Missing unpublished ports in docker ps output
	c.Assert(out, checker.Contains, unpPort1)
	// Missing unpublished ports in docker ps output
	c.Assert(out, checker.Contains, unpPort2)
}

func (s *DockerNetworkSuite) TestDockerNetworkMacInspect(c *check.C) {
	// Verify endpoint MAC address is correctly populated in container's network settings
	nwn := "ov"
	ctn := "bb"

	dockerCmd(c, "network", "create", "-d", dummyNetworkDriver, nwn)
	assertNwIsAvailable(c, nwn)

	dockerCmd(c, "run", "-d", "--net", nwn, "--name", ctn, "busybox", "top")

	mac, err := inspectField(ctn, "NetworkSettings.Networks."+nwn+".MacAddress")
	c.Assert(err, checker.IsNil)
	c.Assert(mac, checker.Equals, "a0:b1:c2:d3:e4:f5")
}

func (s *DockerSuite) TestInspectApiMultipleNetworks(c *check.C) {
	dockerCmd(c, "network", "create", "mybridge1")
	dockerCmd(c, "network", "create", "mybridge2")
	out, _ := dockerCmd(c, "run", "-d", "busybox", "top")
	id := strings.TrimSpace(out)
	c.Assert(waitRun(id), check.IsNil)

	dockerCmd(c, "network", "connect", "mybridge1", id)
	dockerCmd(c, "network", "connect", "mybridge2", id)

	body := getInspectBody(c, "v1.20", id)
	var inspect120 v1p20.ContainerJSON
	err := json.Unmarshal(body, &inspect120)
	c.Assert(err, checker.IsNil)

	versionedIP := inspect120.NetworkSettings.IPAddress

	body = getInspectBody(c, "v1.21", id)
	var inspect121 types.ContainerJSON
	err = json.Unmarshal(body, &inspect121)
	c.Assert(err, checker.IsNil)
	c.Assert(inspect121.NetworkSettings.Networks, checker.HasLen, 3)

	bridge := inspect121.NetworkSettings.Networks["bridge"]
	c.Assert(bridge.IPAddress, checker.Equals, versionedIP)
	c.Assert(bridge.IPAddress, checker.Equals, inspect121.NetworkSettings.IPAddress)
}

func connectContainerToNetworks(c *check.C, d *Daemon, cName string, nws []string) {
	// Run a container on the default network
	out, err := d.Cmd("run", "-d", "--name", cName, "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	// Attach the container to other three networks
	for _, nw := range nws {
		out, err = d.Cmd("network", "create", nw)
		c.Assert(err, checker.IsNil, check.Commentf(out))
		out, err = d.Cmd("network", "connect", nw, cName)
		c.Assert(err, checker.IsNil, check.Commentf(out))
	}
}

func verifyContainerIsConnectedToNetworks(c *check.C, d *Daemon, cName string, nws []string) {
	// Verify container is connected to all three networks
	for _, nw := range nws {
		out, err := d.Cmd("inspect", "-f", fmt.Sprintf("{{.NetworkSettings.Networks.%s}}", nw), cName)
		c.Assert(err, checker.IsNil, check.Commentf(out))
		c.Assert(out, checker.Not(checker.Equals), "<no value>\n")
	}
}

func (s *DockerNetworkSuite) TestDockerNetworkMultipleNetworksGracefulDaemonRestart(c *check.C) {
	cName := "bb"
	nwList := []string{"nw1", "nw2", "nw3"}

	s.d.StartWithBusybox()

	connectContainerToNetworks(c, s.d, cName, nwList)
	verifyContainerIsConnectedToNetworks(c, s.d, cName, nwList)

	// Reload daemon
	s.d.Restart()

	_, err := s.d.Cmd("start", cName)
	c.Assert(err, checker.IsNil)

	verifyContainerIsConnectedToNetworks(c, s.d, cName, nwList)
}

func (s *DockerNetworkSuite) TestDockerNetworkMultipleNetworksUngracefulDaemonRestart(c *check.C) {
	cName := "cc"
	nwList := []string{"nw1", "nw2", "nw3"}

	s.d.StartWithBusybox()

	connectContainerToNetworks(c, s.d, cName, nwList)
	verifyContainerIsConnectedToNetworks(c, s.d, cName, nwList)

	// Kill daemon and restart
	if err := s.d.cmd.Process.Kill(); err != nil {
		c.Fatal(err)
	}
	s.d.Restart()

	// Restart container
	_, err := s.d.Cmd("start", cName)
	c.Assert(err, checker.IsNil)

	verifyContainerIsConnectedToNetworks(c, s.d, cName, nwList)
}

func (s *DockerNetworkSuite) TestDockerNetworkRunNetByID(c *check.C) {
	out, _ := dockerCmd(c, "network", "create", "one")
	dockerCmd(c, "run", "-d", "--net", strings.TrimSpace(out), "busybox", "top")
}

func (s *DockerNetworkSuite) TestDockerNetworkHostModeUngracefulDaemonRestart(c *check.C) {
	testRequires(c, DaemonIsLinux, NotUserNamespace)
	s.d.StartWithBusybox()

	// Run a few containers on host network
	for i := 0; i < 10; i++ {
		cName := fmt.Sprintf("hostc-%d", i)
		out, err := s.d.Cmd("run", "-d", "--name", cName, "--net=host", "--restart=always", "busybox", "top")
		c.Assert(err, checker.IsNil, check.Commentf(out))
	}

	// Kill daemon ungracefully and restart
	if err := s.d.cmd.Process.Kill(); err != nil {
		c.Fatal(err)
	}
	s.d.Restart()

	// make sure all the containers are up and running
	for i := 0; i < 10; i++ {
		cName := fmt.Sprintf("hostc-%d", i)
		runningOut, err := s.d.Cmd("inspect", "--format='{{.State.Running}}'", cName)
		c.Assert(err, checker.IsNil)
		c.Assert(strings.TrimSpace(runningOut), checker.Equals, "true")
	}
}

func (s *DockerNetworkSuite) TestDockerNetworkConnectToHostFromOtherNetwork(c *check.C) {
	dockerCmd(c, "run", "-d", "--name", "container1", "busybox", "top")
	c.Assert(waitRun("container1"), check.IsNil)
	dockerCmd(c, "network", "disconnect", "bridge", "container1")
	out, _, err := dockerCmdWithError("network", "connect", "host", "container1")
	c.Assert(err, checker.NotNil, check.Commentf(out))
	c.Assert(out, checker.Contains, runconfig.ErrConflictHostNetwork.Error())
}

func (s *DockerNetworkSuite) TestDockerNetworkDisconnectFromHost(c *check.C) {
	dockerCmd(c, "run", "-d", "--name", "container1", "--net=host", "busybox", "top")
	c.Assert(waitRun("container1"), check.IsNil)
	out, _, err := dockerCmdWithError("network", "disconnect", "host", "container1")
	c.Assert(err, checker.NotNil, check.Commentf("Should err out disconnect from host"))
	c.Assert(out, checker.Contains, runconfig.ErrConflictHostNetwork.Error())
}

func (s *DockerNetworkSuite) TestDockerNetworkConnectWithPortMapping(c *check.C) {
	dockerCmd(c, "network", "create", "test1")
	dockerCmd(c, "run", "-d", "--name", "c1", "-p", "5000:5000", "busybox", "top")
	c.Assert(waitRun("c1"), check.IsNil)
	dockerCmd(c, "network", "connect", "test1", "c1")
}

func (s *DockerNetworkSuite) TestDockerNetworkConnectWithMac(c *check.C) {
	macAddress := "02:42:ac:11:00:02"
	dockerCmd(c, "network", "create", "mynetwork")
	dockerCmd(c, "run", "--name=test", "-d", "--mac-address", macAddress, "busybox", "top")
	c.Assert(waitRun("test"), check.IsNil)
	mac1, err := inspectField("test", "NetworkSettings.Networks.bridge.MacAddress")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(mac1), checker.Equals, macAddress)
	dockerCmd(c, "network", "connect", "mynetwork", "test")
	mac2, err := inspectField("test", "NetworkSettings.Networks.mynetwork.MacAddress")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(mac2), checker.Not(checker.Equals), strings.TrimSpace(mac1))
}
