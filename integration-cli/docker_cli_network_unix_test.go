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
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/libnetwork/driverapi"
	remoteapi "github.com/docker/libnetwork/drivers/remote/api"
	"github.com/docker/libnetwork/ipamapi"
	remoteipam "github.com/docker/libnetwork/ipams/remote/api"
	"github.com/docker/libnetwork/netlabel"
	"github.com/go-check/check"
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
		if strings.Contains(lines[i], name) {
			return true
		}
	}
	return false
}

func getNwResource(c *check.C, name string) *types.NetworkResource {
	out, _ := dockerCmd(c, "network", "inspect", name)
	nr := types.NetworkResource{}
	err := json.Unmarshal([]byte(out), &nr)
	c.Assert(err, check.IsNil)
	return &nr
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
	containerIP := findContainerIP(c, "test")
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
