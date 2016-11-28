// +build !windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/ipamapi"
	remoteipam "github.com/docker/libnetwork/ipams/remote/api"
	"github.com/go-check/check"
	"github.com/vishvananda/netlink"
)

func (s *DockerSwarmSuite) TestSwarmUpdate(c *check.C) {
	d := s.AddDaemon(c, true, true)

	getSpec := func() swarm.Spec {
		sw := d.getSwarm(c)
		return sw.Spec
	}

	out, err := d.Cmd("swarm", "update", "--cert-expiry", "30h", "--dispatcher-heartbeat", "11s")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))

	spec := getSpec()
	c.Assert(spec.CAConfig.NodeCertExpiry, checker.Equals, 30*time.Hour)
	c.Assert(spec.Dispatcher.HeartbeatPeriod, checker.Equals, 11*time.Second)

	// setting anything under 30m for cert-expiry is not allowed
	out, err = d.Cmd("swarm", "update", "--cert-expiry", "15m")
	c.Assert(err, checker.NotNil)
	c.Assert(out, checker.Contains, "minimum certificate expiry time")
	spec = getSpec()
	c.Assert(spec.CAConfig.NodeCertExpiry, checker.Equals, 30*time.Hour)
}

func (s *DockerSwarmSuite) TestSwarmInit(c *check.C) {
	d := s.AddDaemon(c, false, false)

	getSpec := func() swarm.Spec {
		sw := d.getSwarm(c)
		return sw.Spec
	}

	out, err := d.Cmd("swarm", "init", "--cert-expiry", "30h", "--dispatcher-heartbeat", "11s")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))

	spec := getSpec()
	c.Assert(spec.CAConfig.NodeCertExpiry, checker.Equals, 30*time.Hour)
	c.Assert(spec.Dispatcher.HeartbeatPeriod, checker.Equals, 11*time.Second)

	c.Assert(d.Leave(true), checker.IsNil)
	time.Sleep(500 * time.Millisecond) // https://github.com/docker/swarmkit/issues/1421
	out, err = d.Cmd("swarm", "init")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))

	spec = getSpec()
	c.Assert(spec.CAConfig.NodeCertExpiry, checker.Equals, 90*24*time.Hour)
	c.Assert(spec.Dispatcher.HeartbeatPeriod, checker.Equals, 5*time.Second)
}

func (s *DockerSwarmSuite) TestSwarmInitIPv6(c *check.C) {
	testRequires(c, IPv6)
	d1 := s.AddDaemon(c, false, false)
	out, err := d1.Cmd("swarm", "init", "--listen-addr", "::1")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))

	d2 := s.AddDaemon(c, false, false)
	out, err = d2.Cmd("swarm", "join", "::1")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))

	out, err = d2.Cmd("info")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", out))
	c.Assert(out, checker.Contains, "Swarm: active")
}

func (s *DockerSwarmSuite) TestSwarmIncompatibleDaemon(c *check.C) {
	// init swarm mode and stop a daemon
	d := s.AddDaemon(c, true, true)
	info, err := d.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
	c.Assert(d.Stop(), checker.IsNil)

	// start a daemon with --cluster-store and --cluster-advertise
	err = d.Start("--cluster-store=consul://consuladdr:consulport/some/path", "--cluster-advertise=1.1.1.1:2375")
	c.Assert(err, checker.NotNil)
	content, _ := ioutil.ReadFile(d.logFile.Name())
	c.Assert(string(content), checker.Contains, "--cluster-store and --cluster-advertise daemon configurations are incompatible with swarm mode")

	// start a daemon with --live-restore
	err = d.Start("--live-restore")
	c.Assert(err, checker.NotNil)
	content, _ = ioutil.ReadFile(d.logFile.Name())
	c.Assert(string(content), checker.Contains, "--live-restore daemon configuration is incompatible with swarm mode")
	// restart for teardown
	c.Assert(d.Start(), checker.IsNil)
}

// Test case for #24090
func (s *DockerSwarmSuite) TestSwarmNodeListHostname(c *check.C) {
	d := s.AddDaemon(c, true, true)

	// The first line should contain "HOSTNAME"
	out, err := d.Cmd("node", "ls")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.Split(out, "\n")[0], checker.Contains, "HOSTNAME")
}

func (s *DockerSwarmSuite) TestSwarmServiceTemplatingHostname(c *check.C) {
	d := s.AddDaemon(c, true, true)

	out, err := d.Cmd("service", "create", "--name", "test", "--hostname", "{{.Service.Name}}-{{.Task.Slot}}", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 1)

	containers := d.activeContainers()
	out, err = d.Cmd("inspect", "--type", "container", "--format", "{{.Config.Hostname}}", containers[0])
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(strings.Split(out, "\n")[0], checker.Equals, "test-1", check.Commentf("hostname with templating invalid"))
}

// Test case for #24270
func (s *DockerSwarmSuite) TestSwarmServiceListFilter(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name1 := "redis-cluster-md5"
	name2 := "redis-cluster"
	name3 := "other-cluster"
	out, err := d.Cmd("service", "create", "--name", name1, "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	out, err = d.Cmd("service", "create", "--name", name2, "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	out, err = d.Cmd("service", "create", "--name", name3, "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	filter1 := "name=redis-cluster-md5"
	filter2 := "name=redis-cluster"

	// We search checker.Contains with `name+" "` to prevent prefix only.
	out, err = d.Cmd("service", "ls", "--filter", filter1)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name1+" ")
	c.Assert(out, checker.Not(checker.Contains), name2+" ")
	c.Assert(out, checker.Not(checker.Contains), name3+" ")

	out, err = d.Cmd("service", "ls", "--filter", filter2)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name1+" ")
	c.Assert(out, checker.Contains, name2+" ")
	c.Assert(out, checker.Not(checker.Contains), name3+" ")

	out, err = d.Cmd("service", "ls")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name1+" ")
	c.Assert(out, checker.Contains, name2+" ")
	c.Assert(out, checker.Contains, name3+" ")
}

func (s *DockerSwarmSuite) TestSwarmNodeListFilter(c *check.C) {
	d := s.AddDaemon(c, true, true)

	out, err := d.Cmd("node", "inspect", "--format", "{{ .Description.Hostname }}", "self")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")
	name := strings.TrimSpace(out)

	filter := "name=" + name[:4]

	out, err = d.Cmd("node", "ls", "--filter", filter)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name)

	out, err = d.Cmd("node", "ls", "--filter", "name=none")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Not(checker.Contains), name)
}

func (s *DockerSwarmSuite) TestSwarmNodeTaskListFilter(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name := "redis-cluster-md5"
	out, err := d.Cmd("service", "create", "--name", name, "--replicas=3", "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 3)

	filter := "name=redis-cluster"

	out, err = d.Cmd("node", "ps", "--filter", filter, "self")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name+".1")
	c.Assert(out, checker.Contains, name+".2")
	c.Assert(out, checker.Contains, name+".3")

	out, err = d.Cmd("node", "ps", "--filter", "name=none", "self")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Not(checker.Contains), name+".1")
	c.Assert(out, checker.Not(checker.Contains), name+".2")
	c.Assert(out, checker.Not(checker.Contains), name+".3")
}

// Test case for #25375
func (s *DockerSwarmSuite) TestSwarmPublishAdd(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name := "top"
	out, err := d.Cmd("service", "create", "--name", name, "--label", "x=y", "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	out, err = d.Cmd("service", "update", "--publish-add", "80:80", name)
	c.Assert(err, checker.IsNil)

	out, err = d.cmdRetryOutOfSequence("service", "update", "--publish-add", "80:80", name)
	c.Assert(err, checker.IsNil)

	out, err = d.cmdRetryOutOfSequence("service", "update", "--publish-add", "80:80", "--publish-add", "80:20", name)
	c.Assert(err, checker.NotNil)

	out, err = d.Cmd("service", "inspect", "--format", "{{ .Spec.EndpointSpec.Ports }}", name)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Equals, "[{ tcp 80 80 ingress}]")
}

func (s *DockerSwarmSuite) TestSwarmServiceWithGroup(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name := "top"
	out, err := d.Cmd("service", "create", "--name", name, "--user", "root:root", "--group", "wheel", "--group", "audio", "--group", "staff", "--group", "777", "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 1)

	out, err = d.Cmd("ps", "-q")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	container := strings.TrimSpace(out)

	out, err = d.Cmd("exec", container, "id")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Equals, "uid=0(root) gid=0(root) groups=10(wheel),29(audio),50(staff),777")
}

func (s *DockerSwarmSuite) TestSwarmContainerAutoStart(c *check.C) {
	d := s.AddDaemon(c, true, true)

	out, err := d.Cmd("network", "create", "--attachable", "-d", "overlay", "foo")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	out, err = d.Cmd("run", "-id", "--restart=always", "--net=foo", "--name=test", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	out, err = d.Cmd("ps", "-q")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	d.Restart()

	out, err = d.Cmd("ps", "-q")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")
}

func (s *DockerSwarmSuite) TestSwarmContainerEndpointOptions(c *check.C) {
	d := s.AddDaemon(c, true, true)

	out, err := d.Cmd("network", "create", "--attachable", "-d", "overlay", "foo")
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	_, err = d.Cmd("run", "-d", "--net=foo", "--name=first", "--net-alias=first-alias", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	_, err = d.Cmd("run", "-d", "--net=foo", "--name=second", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	// ping first container and its alias
	_, err = d.Cmd("exec", "second", "ping", "-c", "1", "first")
	c.Assert(err, check.IsNil, check.Commentf(out))
	_, err = d.Cmd("exec", "second", "ping", "-c", "1", "first-alias")
	c.Assert(err, check.IsNil, check.Commentf(out))
}

func (s *DockerSwarmSuite) TestSwarmContainerAttachByNetworkId(c *check.C) {
	d := s.AddDaemon(c, true, true)

	out, err := d.Cmd("network", "create", "--attachable", "-d", "overlay", "testnet")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")
	networkID := strings.TrimSpace(out)

	out, err = d.Cmd("run", "-d", "--net", networkID, "busybox", "top")
	c.Assert(err, checker.IsNil)
	cID := strings.TrimSpace(out)
	d.waitRun(cID)

	_, err = d.Cmd("rm", "-f", cID)
	c.Assert(err, checker.IsNil)

	out, err = d.Cmd("network", "rm", "testnet")
	c.Assert(err, checker.IsNil)

	checkNetwork := func(*check.C) (interface{}, check.CommentInterface) {
		out, err := d.Cmd("network", "ls")
		c.Assert(err, checker.IsNil)
		return out, nil
	}

	waitAndAssert(c, 3*time.Second, checkNetwork, checker.Not(checker.Contains), "testnet")
}

func (s *DockerSwarmSuite) TestSwarmRemoveInternalNetwork(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name := "ingress"
	out, err := d.Cmd("network", "rm", name)
	c.Assert(err, checker.NotNil)
	c.Assert(strings.TrimSpace(out), checker.Contains, name)
	c.Assert(strings.TrimSpace(out), checker.Contains, "is a pre-defined network and cannot be removed")
}

// Test case for #24108, also the case from:
// https://github.com/docker/docker/pull/24620#issuecomment-233715656
func (s *DockerSwarmSuite) TestSwarmTaskListFilter(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name := "redis-cluster-md5"
	out, err := d.Cmd("service", "create", "--name", name, "--replicas=3", "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	filter := "name=redis-cluster"

	checkNumTasks := func(*check.C) (interface{}, check.CommentInterface) {
		out, err := d.Cmd("service", "ps", "--filter", filter, name)
		c.Assert(err, checker.IsNil)
		return len(strings.Split(out, "\n")) - 2, nil // includes header and nl in last line
	}

	// wait until all tasks have been created
	waitAndAssert(c, defaultReconciliationTimeout, checkNumTasks, checker.Equals, 3)

	out, err = d.Cmd("service", "ps", "--filter", filter, name)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name+".1")
	c.Assert(out, checker.Contains, name+".2")
	c.Assert(out, checker.Contains, name+".3")

	out, err = d.Cmd("service", "ps", "--filter", "name="+name+".1", name)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name+".1")
	c.Assert(out, checker.Not(checker.Contains), name+".2")
	c.Assert(out, checker.Not(checker.Contains), name+".3")

	out, err = d.Cmd("service", "ps", "--filter", "name=none", name)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Not(checker.Contains), name+".1")
	c.Assert(out, checker.Not(checker.Contains), name+".2")
	c.Assert(out, checker.Not(checker.Contains), name+".3")

	name = "redis-cluster-sha1"
	out, err = d.Cmd("service", "create", "--name", name, "--mode=global", "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	waitAndAssert(c, defaultReconciliationTimeout, checkNumTasks, checker.Equals, 1)

	filter = "name=redis-cluster"
	out, err = d.Cmd("service", "ps", "--filter", filter, name)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name)

	out, err = d.Cmd("service", "ps", "--filter", "name="+name, name)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, name)

	out, err = d.Cmd("service", "ps", "--filter", "name=none", name)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Not(checker.Contains), name)
}

func (s *DockerSwarmSuite) TestPsListContainersFilterIsTask(c *check.C) {
	d := s.AddDaemon(c, true, true)

	// Create a bare container
	out, err := d.Cmd("run", "-d", "--name=bare-container", "busybox", "top")
	c.Assert(err, checker.IsNil)
	bareID := strings.TrimSpace(out)[:12]
	// Create a service
	name := "busybox-top"
	out, err = d.Cmd("service", "create", "--name", name, "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	// make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.checkServiceRunningTasks(name), checker.Equals, 1)

	// Filter non-tasks
	out, err = d.Cmd("ps", "-a", "-q", "--filter=is-task=false")
	c.Assert(err, checker.IsNil)
	psOut := strings.TrimSpace(out)
	c.Assert(psOut, checker.Equals, bareID, check.Commentf("Expected id %s, got %s for is-task label, output %q", bareID, psOut, out))

	// Filter tasks
	out, err = d.Cmd("ps", "-a", "-q", "--filter=is-task=true")
	c.Assert(err, checker.IsNil)
	lines := strings.Split(strings.Trim(out, "\n "), "\n")
	c.Assert(lines, checker.HasLen, 1)
	c.Assert(lines[0], checker.Not(checker.Equals), bareID, check.Commentf("Expected not %s, but got it for is-task label, output %q", bareID, out))
}

const globalNetworkPlugin = "global-network-plugin"
const globalIPAMPlugin = "global-ipam-plugin"

func (s *DockerSwarmSuite) SetUpSuite(c *check.C) {
	mux := http.NewServeMux()
	s.server = httptest.NewServer(mux)
	c.Assert(s.server, check.NotNil, check.Commentf("Failed to start an HTTP Server"))
	setupRemoteGlobalNetworkPlugin(c, mux, s.server.URL, globalNetworkPlugin, globalIPAMPlugin)
}

func setupRemoteGlobalNetworkPlugin(c *check.C, mux *http.ServeMux, url, netDrv, ipamDrv string) {

	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintf(w, `{"Implements": ["%s", "%s"]}`, driverapi.NetworkPluginEndpointType, ipamapi.PluginEndpointType)
	})

	// Network driver implementation
	mux.HandleFunc(fmt.Sprintf("/%s.GetCapabilities", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintf(w, `{"Scope":"global"}`)
	})

	mux.HandleFunc(fmt.Sprintf("/%s.AllocateNetwork", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		err := json.NewDecoder(r.Body).Decode(&remoteDriverNetworkRequest)
		if err != nil {
			http.Error(w, "Unable to decode JSON payload: "+err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintf(w, "null")
	})

	mux.HandleFunc(fmt.Sprintf("/%s.FreeNetwork", driverapi.NetworkPluginEndpointType), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.plugins.v1+json")
		fmt.Fprintf(w, "null")
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

	// IPAM Driver implementation
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
		// make sure libnetwork is now asking to release the expected address from the expected poolid
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

	fileName := fmt.Sprintf("/etc/docker/plugins/%s.spec", netDrv)
	err = ioutil.WriteFile(fileName, []byte(url), 0644)
	c.Assert(err, checker.IsNil)

	ipamFileName := fmt.Sprintf("/etc/docker/plugins/%s.spec", ipamDrv)
	err = ioutil.WriteFile(ipamFileName, []byte(url), 0644)
	c.Assert(err, checker.IsNil)
}

func (s *DockerSwarmSuite) TestSwarmNetworkPlugin(c *check.C) {
	d := s.AddDaemon(c, true, true)

	out, err := d.Cmd("network", "create", "-d", globalNetworkPlugin, "foo")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	name := "top"
	out, err = d.Cmd("service", "create", "--name", name, "--network", "foo", "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	out, err = d.Cmd("service", "inspect", "--format", "{{range .Spec.Networks}}{{.Target}}{{end}}", name)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Equals, "foo")
}

// Test case for #24712
func (s *DockerSwarmSuite) TestSwarmServiceEnvFile(c *check.C) {
	d := s.AddDaemon(c, true, true)

	path := filepath.Join(d.folder, "env.txt")
	err := ioutil.WriteFile(path, []byte("VAR1=A\nVAR2=A\n"), 0644)
	c.Assert(err, checker.IsNil)

	name := "worker"
	out, err := d.Cmd("service", "create", "--env-file", path, "--env", "VAR1=B", "--env", "VAR1=C", "--env", "VAR2=", "--env", "VAR2", "--name", name, "busybox", "top")
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Not(checker.Equals), "")

	// The complete env is [VAR1=A VAR2=A VAR1=B VAR1=C VAR2= VAR2] and duplicates will be removed => [VAR1=C VAR2]
	out, err = d.Cmd("inspect", "--format", "{{ .Spec.TaskTemplate.ContainerSpec.Env }}", name)
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, "[VAR1=C VAR2]")
}

func (s *DockerSwarmSuite) TestSwarmServiceTTY(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name := "top"

	ttyCheck := "if [ -t 0 ]; then echo TTY > /status && top; else echo none > /status && top; fi"

	// Without --tty
	expectedOutput := "none"
	out, err := d.Cmd("service", "create", "--name", name, "busybox", "sh", "-c", ttyCheck)
	c.Assert(err, checker.IsNil)

	// Make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 1)

	// We need to get the container id.
	out, err = d.Cmd("ps", "-a", "-q", "--no-trunc")
	c.Assert(err, checker.IsNil)
	id := strings.TrimSpace(out)

	out, err = d.Cmd("exec", id, "cat", "/status")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, expectedOutput, check.Commentf("Expected '%s', but got %q", expectedOutput, out))

	// Remove service
	out, err = d.Cmd("service", "rm", name)
	c.Assert(err, checker.IsNil)
	// Make sure container has been destroyed.
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 0)

	// With --tty
	expectedOutput = "TTY"
	out, err = d.Cmd("service", "create", "--name", name, "--tty", "busybox", "sh", "-c", ttyCheck)
	c.Assert(err, checker.IsNil)

	// Make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 1)

	// We need to get the container id.
	out, err = d.Cmd("ps", "-a", "-q", "--no-trunc")
	c.Assert(err, checker.IsNil)
	id = strings.TrimSpace(out)

	out, err = d.Cmd("exec", id, "cat", "/status")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, expectedOutput, check.Commentf("Expected '%s', but got %q", expectedOutput, out))
}

func (s *DockerSwarmSuite) TestSwarmServiceTTYUpdate(c *check.C) {
	d := s.AddDaemon(c, true, true)

	// Create a service
	name := "top"
	_, err := d.Cmd("service", "create", "--name", name, "busybox", "top")
	c.Assert(err, checker.IsNil)

	// Make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 1)

	out, err := d.Cmd("service", "inspect", "--format", "{{ .Spec.TaskTemplate.ContainerSpec.TTY }}", name)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Equals, "false")

	_, err = d.Cmd("service", "update", "--tty", name)
	c.Assert(err, checker.IsNil)

	out, err = d.Cmd("service", "inspect", "--format", "{{ .Spec.TaskTemplate.ContainerSpec.TTY }}", name)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Equals, "true")
}

func (s *DockerSwarmSuite) TestDNSConfig(c *check.C) {
	d := s.AddDaemon(c, true, true)

	// Create a service
	name := "top"
	_, err := d.Cmd("service", "create", "--name", name, "--dns=1.2.3.4", "--dns-search=example.com", "--dns-option=timeout:3", "busybox", "top")
	c.Assert(err, checker.IsNil)

	// Make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 1)

	// We need to get the container id.
	out, err := d.Cmd("ps", "-a", "-q", "--no-trunc")
	c.Assert(err, checker.IsNil)
	id := strings.TrimSpace(out)

	// Compare against expected output.
	expectedOutput1 := "nameserver 1.2.3.4"
	expectedOutput2 := "search example.com"
	expectedOutput3 := "options timeout:3"
	out, err = d.Cmd("exec", id, "cat", "/etc/resolv.conf")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, expectedOutput1, check.Commentf("Expected '%s', but got %q", expectedOutput1, out))
	c.Assert(out, checker.Contains, expectedOutput2, check.Commentf("Expected '%s', but got %q", expectedOutput2, out))
	c.Assert(out, checker.Contains, expectedOutput3, check.Commentf("Expected '%s', but got %q", expectedOutput3, out))
}

func (s *DockerSwarmSuite) TestDNSConfigUpdate(c *check.C) {
	d := s.AddDaemon(c, true, true)

	// Create a service
	name := "top"
	_, err := d.Cmd("service", "create", "--name", name, "busybox", "top")
	c.Assert(err, checker.IsNil)

	// Make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 1)

	_, err = d.Cmd("service", "update", "--dns-add=1.2.3.4", "--dns-search-add=example.com", "--dns-option-add=timeout:3", name)
	c.Assert(err, checker.IsNil)

	out, err := d.Cmd("service", "inspect", "--format", "{{ .Spec.TaskTemplate.ContainerSpec.DNSConfig }}", name)
	c.Assert(err, checker.IsNil)
	c.Assert(strings.TrimSpace(out), checker.Equals, "{[1.2.3.4] [example.com] [timeout:3]}")
}

func getNodeStatus(c *check.C, d *SwarmDaemon) swarm.LocalNodeState {
	info, err := d.info()
	c.Assert(err, checker.IsNil)
	return info.LocalNodeState
}

func (s *DockerSwarmSuite) TestSwarmInitLocked(c *check.C) {
	d := s.AddDaemon(c, false, false)

	outs, err := d.Cmd("swarm", "init", "--autolock")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", outs))

	c.Assert(outs, checker.Contains, "docker swarm unlock")

	var unlockKey string
	for _, line := range strings.Split(outs, "\n") {
		if strings.Contains(line, "SWMKEY") {
			unlockKey = strings.TrimSpace(line)
			break
		}
	}

	c.Assert(unlockKey, checker.Not(checker.Equals), "")

	outs, err = d.Cmd("swarm", "unlock-key", "-q")
	c.Assert(outs, checker.Equals, unlockKey+"\n")

	c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateActive)

	c.Assert(d.Restart(), checker.IsNil)

	c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateLocked)

	cmd := d.command("swarm", "unlock")
	cmd.Stdin = bytes.NewBufferString("wrong-secret-key")
	out, err := cmd.CombinedOutput()
	c.Assert(err, checker.NotNil, check.Commentf("out: %v", string(out)))
	c.Assert(string(out), checker.Contains, "invalid key")

	c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateLocked)

	cmd = d.command("swarm", "unlock")
	cmd.Stdin = bytes.NewBufferString(unlockKey)
	out, err = cmd.CombinedOutput()
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", string(out)))

	c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateActive)

	outs, err = d.Cmd("node", "ls")
	c.Assert(err, checker.IsNil)
	c.Assert(outs, checker.Not(checker.Contains), "Swarm is encrypted and needs to be unlocked")

	outs, err = d.Cmd("swarm", "update", "--autolock=false")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", outs))

	// Wait for autolock to be turned off
	c.Assert(d.Restart(), checker.IsNil)
	status := getNodeStatus(c, d)
	if status == swarm.LocalNodeStateLocked {
		// it must not have updated in time - unlock, wait 3 seconds, and try again
		cmd := d.command("swarm", "unlock")
		cmd.Stdin = bytes.NewBufferString(unlockKey)
		out, err := cmd.CombinedOutput()
		c.Assert(err, checker.IsNil, check.Commentf("out: %v", string(out)))

		c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateActive)

		time.Sleep(3 * time.Second)
		c.Assert(d.Restart(), checker.IsNil)
	}

	c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateActive)

	outs, err = d.Cmd("node", "ls")
	c.Assert(err, checker.IsNil)
	c.Assert(outs, checker.Not(checker.Contains), "Swarm is encrypted and needs to be unlocked")
}

func (s *DockerSwarmSuite) TestSwarmLeaveLocked(c *check.C) {
	d := s.AddDaemon(c, false, false)

	outs, err := d.Cmd("swarm", "init", "--autolock")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", outs))

	c.Assert(d.Restart("--swarm-default-advertise-addr=lo"), checker.IsNil)

	info, err := d.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateLocked)

	outs, _ = d.Cmd("node", "ls")
	c.Assert(outs, checker.Contains, "Swarm is encrypted and needs to be unlocked")

	outs, err = d.Cmd("swarm", "leave", "--force")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", outs))

	info, err = d.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateInactive)

	outs, err = d.Cmd("swarm", "init")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", outs))

	info, err = d.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
}

func (s *DockerSwarmSuite) TestSwarmLockUnlockCluster(c *check.C) {
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, true)
	d3 := s.AddDaemon(c, true, true)

	// they start off unlocked
	c.Assert(d2.Restart(), checker.IsNil)
	c.Assert(getNodeStatus(c, d2), checker.Equals, swarm.LocalNodeStateActive)

	// stop this one so it does not get autolock info
	c.Assert(d2.Stop(), checker.IsNil)

	// enable autolock
	outs, err := d1.Cmd("swarm", "update", "--autolock")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", outs))

	c.Assert(outs, checker.Contains, "docker swarm unlock")

	var unlockKey string
	for _, line := range strings.Split(outs, "\n") {
		if strings.Contains(line, "SWMKEY") {
			unlockKey = strings.TrimSpace(line)
			break
		}
	}

	c.Assert(unlockKey, checker.Not(checker.Equals), "")

	outs, err = d1.Cmd("swarm", "unlock-key", "-q")
	c.Assert(outs, checker.Equals, unlockKey+"\n")

	// The ones that got the cluster update should be set to locked
	for _, d := range []*SwarmDaemon{d1, d3} {
		c.Assert(d.Restart(), checker.IsNil)
		c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateLocked)

		cmd := d.command("swarm", "unlock")
		cmd.Stdin = bytes.NewBufferString(unlockKey)
		out, err := cmd.CombinedOutput()
		c.Assert(err, checker.IsNil, check.Commentf("out: %v", string(out)))
		c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateActive)
	}

	// d2 never got the cluster update, so it is still set to unlocked
	c.Assert(d2.Start(), checker.IsNil)
	c.Assert(getNodeStatus(c, d2), checker.Equals, swarm.LocalNodeStateActive)

	// d2 is now set to lock
	c.Assert(d2.Restart(), checker.IsNil)
	status := getNodeStatus(c, d2)
	if status == swarm.LocalNodeStateActive {
		// it must not have updated in time - wait 3 seconds, and try again
		time.Sleep(3 * time.Second)
		c.Assert(d2.Restart(), checker.IsNil)
	}
	c.Assert(getNodeStatus(c, d2), checker.Equals, swarm.LocalNodeStateLocked)

	// leave it locked, and set the cluster to no longer autolock
	outs, err = d1.Cmd("swarm", "update", "--autolock=false")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", outs))

	// the ones that got the update are now set to unlocked
	for _, d := range []*SwarmDaemon{d1, d3} {
		c.Assert(d.Restart(), checker.IsNil)
		status := getNodeStatus(c, d)
		if status == swarm.LocalNodeStateLocked {
			// it must not have updated to be unlocked in time - unlock, wait 3 seconds, and try again
			cmd := d.command("swarm", "unlock")
			cmd.Stdin = bytes.NewBufferString(unlockKey)
			out, err := cmd.CombinedOutput()
			c.Assert(err, checker.IsNil, check.Commentf("out: %v", string(out)))

			c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateActive)

			time.Sleep(3 * time.Second)
			c.Assert(d.Restart(), checker.IsNil)
		}

		c.Assert(d.Restart(), checker.IsNil)
		c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateActive)
	}

	// d2 still locked
	c.Assert(getNodeStatus(c, d2), checker.Equals, swarm.LocalNodeStateLocked)

	// unlock it
	cmd := d2.command("swarm", "unlock")
	cmd.Stdin = bytes.NewBufferString(unlockKey)
	out, err := cmd.CombinedOutput()
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", string(out)))
	c.Assert(getNodeStatus(c, d2), checker.Equals, swarm.LocalNodeStateActive)

	// once it's caught up, d2 is set to not be locked
	c.Assert(d2.Restart(), checker.IsNil)
	c.Assert(getNodeStatus(c, d2), checker.Equals, swarm.LocalNodeStateActive)

	// managers who join now are also unlocked
	d4 := s.AddDaemon(c, true, true)
	c.Assert(d4.Restart(), checker.IsNil)
	c.Assert(getNodeStatus(c, d4), checker.Equals, swarm.LocalNodeStateActive)
}

func (s *DockerSwarmSuite) TestSwarmJoinPromoteLocked(c *check.C) {
	d1 := s.AddDaemon(c, true, true)

	// enable autolock
	outs, err := d1.Cmd("swarm", "update", "--autolock")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", outs))

	c.Assert(outs, checker.Contains, "docker swarm unlock")

	var unlockKey string
	for _, line := range strings.Split(outs, "\n") {
		if strings.Contains(line, "SWMKEY") {
			unlockKey = strings.TrimSpace(line)
			break
		}
	}

	c.Assert(unlockKey, checker.Not(checker.Equals), "")

	outs, err = d1.Cmd("swarm", "unlock-key", "-q")
	c.Assert(outs, checker.Equals, unlockKey+"\n")

	// joined workers start off unlocked
	d2 := s.AddDaemon(c, true, false)
	c.Assert(d2.Restart(), checker.IsNil)
	c.Assert(getNodeStatus(c, d2), checker.Equals, swarm.LocalNodeStateActive)

	// promote worker
	outs, err = d1.Cmd("node", "promote", d2.Info.NodeID)
	c.Assert(err, checker.IsNil)
	c.Assert(outs, checker.Contains, "promoted to a manager in the swarm")

	// join new manager node
	d3 := s.AddDaemon(c, true, true)

	// both new nodes are locked
	for _, d := range []*SwarmDaemon{d2, d3} {
		c.Assert(d.Restart(), checker.IsNil)
		status := getNodeStatus(c, d)
		if status == swarm.LocalNodeStateActive {
			// it must not have updated in time - wait 3 seconds, and try again
			time.Sleep(3 * time.Second)
			c.Assert(d.Restart(), checker.IsNil)
		}
		c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateLocked)

		cmd := d.command("swarm", "unlock")
		cmd.Stdin = bytes.NewBufferString(unlockKey)
		out, err := cmd.CombinedOutput()
		c.Assert(err, checker.IsNil, check.Commentf("out: %v", string(out)))
		c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateActive)
	}

	// demote manager back to worker - workers are not locked
	outs, err = d1.Cmd("node", "demote", d3.Info.NodeID)
	c.Assert(err, checker.IsNil)
	c.Assert(outs, checker.Contains, "demoted in the swarm")

	// verify that it's been demoted
	out, err := d1.Cmd("node", "ls", "--filter", "id="+d3.Info.NodeID)
	c.Assert(err, checker.IsNil)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	c.Assert(len(lines), checker.GreaterThan, 0)
	columns := strings.Fields(lines[len(lines)-1])
	c.Assert(columns, checker.HasLen, 4) // if it was a manager it'd have a manager status field

	c.Assert(d3.Restart(), checker.IsNil)
	status := getNodeStatus(c, d3)
	if status == swarm.LocalNodeStateLocked {
		// it must not have updated in time - unlock, wait 3 seconds, and try again
		cmd := d3.command("swarm", "unlock")
		cmd.Stdin = bytes.NewBufferString(unlockKey)
		out, err := cmd.CombinedOutput()
		c.Assert(err, checker.IsNil, check.Commentf("out: %v", string(out)))

		c.Assert(getNodeStatus(c, d3), checker.Equals, swarm.LocalNodeStateActive)

		time.Sleep(3 * time.Second)
		c.Assert(d3.Restart(), checker.IsNil)
	}
	c.Assert(getNodeStatus(c, d3), checker.Equals, swarm.LocalNodeStateActive)
}

func (s *DockerSwarmSuite) TestSwarmRotateUnlockKey(c *check.C) {
	d := s.AddDaemon(c, true, true)

	outs, err := d.Cmd("swarm", "update", "--autolock")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", outs))

	c.Assert(outs, checker.Contains, "docker swarm unlock")

	var unlockKey string
	for _, line := range strings.Split(outs, "\n") {
		if strings.Contains(line, "SWMKEY") {
			unlockKey = strings.TrimSpace(line)
			break
		}
	}

	c.Assert(unlockKey, checker.Not(checker.Equals), "")

	outs, err = d.Cmd("swarm", "unlock-key", "-q")
	c.Assert(outs, checker.Equals, unlockKey+"\n")

	// Rotate multiple times
	for i := 0; i != 3; i++ {
		outs, err = d.Cmd("swarm", "unlock-key", "-q", "--rotate")
		c.Assert(err, checker.IsNil, check.Commentf("out: %v", outs))
		// Strip \n
		newUnlockKey := outs[:len(outs)-1]
		c.Assert(newUnlockKey, checker.Not(checker.Equals), "")
		c.Assert(newUnlockKey, checker.Not(checker.Equals), unlockKey)

		c.Assert(d.Restart(), checker.IsNil)
		c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateLocked)

		outs, _ = d.Cmd("node", "ls")
		c.Assert(outs, checker.Contains, "Swarm is encrypted and needs to be unlocked")

		cmd := d.command("swarm", "unlock")
		cmd.Stdin = bytes.NewBufferString(unlockKey)
		out, err := cmd.CombinedOutput()

		if err == nil {
			// On occasion, the daemon may not have finished
			// rotating the KEK before restarting. The test is
			// intentionally written to explore this behavior.
			// When this happens, unlocking with the old key will
			// succeed. If we wait for the rotation to happen and
			// restart again, the new key should be required this
			// time.

			time.Sleep(3 * time.Second)

			c.Assert(d.Restart(), checker.IsNil)

			cmd = d.command("swarm", "unlock")
			cmd.Stdin = bytes.NewBufferString(unlockKey)
			out, err = cmd.CombinedOutput()
		}
		c.Assert(err, checker.NotNil, check.Commentf("out: %v", string(out)))
		c.Assert(string(out), checker.Contains, "invalid key")

		outs, _ = d.Cmd("node", "ls")
		c.Assert(outs, checker.Contains, "Swarm is encrypted and needs to be unlocked")

		cmd = d.command("swarm", "unlock")
		cmd.Stdin = bytes.NewBufferString(newUnlockKey)
		out, err = cmd.CombinedOutput()
		c.Assert(err, checker.IsNil, check.Commentf("out: %v", string(out)))

		c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateActive)

		outs, err = d.Cmd("node", "ls")
		c.Assert(err, checker.IsNil)
		c.Assert(outs, checker.Not(checker.Contains), "Swarm is encrypted and needs to be unlocked")

		unlockKey = newUnlockKey
	}
}

// This differs from `TestSwarmRotateUnlockKey` because that one rotates a single node, which is the leader.
// This one keeps the leader up, and asserts that other manager nodes in the cluster also have their unlock
// key rotated.
func (s *DockerSwarmSuite) TestSwarmClusterRotateUnlockKey(c *check.C) {
	d1 := s.AddDaemon(c, true, true) // leader - don't restart this one, we don't want leader election delays
	d2 := s.AddDaemon(c, true, true)
	d3 := s.AddDaemon(c, true, true)

	outs, err := d1.Cmd("swarm", "update", "--autolock")
	c.Assert(err, checker.IsNil, check.Commentf("out: %v", outs))

	c.Assert(outs, checker.Contains, "docker swarm unlock")

	var unlockKey string
	for _, line := range strings.Split(outs, "\n") {
		if strings.Contains(line, "SWMKEY") {
			unlockKey = strings.TrimSpace(line)
			break
		}
	}

	c.Assert(unlockKey, checker.Not(checker.Equals), "")

	outs, err = d1.Cmd("swarm", "unlock-key", "-q")
	c.Assert(outs, checker.Equals, unlockKey+"\n")

	// Rotate multiple times
	for i := 0; i != 3; i++ {
		outs, err = d1.Cmd("swarm", "unlock-key", "-q", "--rotate")
		c.Assert(err, checker.IsNil, check.Commentf("out: %v", outs))
		// Strip \n
		newUnlockKey := outs[:len(outs)-1]
		c.Assert(newUnlockKey, checker.Not(checker.Equals), "")
		c.Assert(newUnlockKey, checker.Not(checker.Equals), unlockKey)

		c.Assert(d2.Restart(), checker.IsNil)
		c.Assert(d3.Restart(), checker.IsNil)

		for _, d := range []*SwarmDaemon{d2, d3} {
			c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateLocked)

			outs, _ := d.Cmd("node", "ls")
			c.Assert(outs, checker.Contains, "Swarm is encrypted and needs to be unlocked")

			cmd := d.command("swarm", "unlock")
			cmd.Stdin = bytes.NewBufferString(unlockKey)
			out, err := cmd.CombinedOutput()

			if err == nil {
				// On occasion, the daemon may not have finished
				// rotating the KEK before restarting. The test is
				// intentionally written to explore this behavior.
				// When this happens, unlocking with the old key will
				// succeed. If we wait for the rotation to happen and
				// restart again, the new key should be required this
				// time.

				time.Sleep(3 * time.Second)

				c.Assert(d.Restart(), checker.IsNil)

				cmd = d.command("swarm", "unlock")
				cmd.Stdin = bytes.NewBufferString(unlockKey)
				out, err = cmd.CombinedOutput()
			}
			c.Assert(err, checker.NotNil, check.Commentf("out: %v", string(out)))
			c.Assert(string(out), checker.Contains, "invalid key")

			outs, _ = d.Cmd("node", "ls")
			c.Assert(outs, checker.Contains, "Swarm is encrypted and needs to be unlocked")

			cmd = d.command("swarm", "unlock")
			cmd.Stdin = bytes.NewBufferString(newUnlockKey)
			out, err = cmd.CombinedOutput()
			c.Assert(err, checker.IsNil, check.Commentf("out: %v", string(out)))

			c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateActive)

			outs, err = d.Cmd("node", "ls")
			c.Assert(err, checker.IsNil)
			c.Assert(outs, checker.Not(checker.Contains), "Swarm is encrypted and needs to be unlocked")
		}

		unlockKey = newUnlockKey
	}
}

func (s *DockerSwarmSuite) TestSwarmAlternateLockUnlock(c *check.C) {
	d := s.AddDaemon(c, true, true)

	var unlockKey string
	for i := 0; i < 2; i++ {
		// set to lock
		outs, err := d.Cmd("swarm", "update", "--autolock")
		c.Assert(err, checker.IsNil, check.Commentf("out: %v", outs))
		c.Assert(outs, checker.Contains, "docker swarm unlock")

		for _, line := range strings.Split(outs, "\n") {
			if strings.Contains(line, "SWMKEY") {
				unlockKey = strings.TrimSpace(line)
				break
			}
		}

		c.Assert(unlockKey, checker.Not(checker.Equals), "")

		c.Assert(d.Restart(), checker.IsNil)
		status := getNodeStatus(c, d)
		if status == swarm.LocalNodeStateActive {
			// hasn't updated yet - wait and try again
			time.Sleep(3 * time.Second)
			c.Assert(d.Restart(), checker.IsNil)
			status = getNodeStatus(c, d)
		}
		c.Assert(status, checker.Equals, swarm.LocalNodeStateLocked)

		cmd := d.command("swarm", "unlock")
		cmd.Stdin = bytes.NewBufferString(unlockKey)
		out, err := cmd.CombinedOutput()
		c.Assert(err, checker.IsNil, check.Commentf("out: %v", string(out)))

		c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateActive)

		outs, err = d.Cmd("swarm", "update", "--autolock=false")
		c.Assert(err, checker.IsNil, check.Commentf("out: %v", outs))

		c.Assert(d.Restart(), checker.IsNil)
		status = getNodeStatus(c, d)
		if status == swarm.LocalNodeStateLocked {
			// it must not have updated to be unlocked in time - unlock, wait 3 seconds, and try again
			cmd := d.command("swarm", "unlock")
			cmd.Stdin = bytes.NewBufferString(unlockKey)
			out, err := cmd.CombinedOutput()
			c.Assert(err, checker.IsNil, check.Commentf("out: %v", string(out)))

			c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateActive)

			time.Sleep(3 * time.Second)
			c.Assert(d.Restart(), checker.IsNil)
			status = getNodeStatus(c, d)
		}
		c.Assert(getNodeStatus(c, d), checker.Equals, swarm.LocalNodeStateActive)
	}
}

func (s *DockerSwarmSuite) TestExtraHosts(c *check.C) {
	d := s.AddDaemon(c, true, true)

	// Create a service
	name := "top"
	_, err := d.Cmd("service", "create", "--name", name, "--host=example.com:1.2.3.4", "busybox", "top")
	c.Assert(err, checker.IsNil)

	// Make sure task has been deployed.
	waitAndAssert(c, defaultReconciliationTimeout, d.checkActiveContainerCount, checker.Equals, 1)

	// We need to get the container id.
	out, err := d.Cmd("ps", "-a", "-q", "--no-trunc")
	c.Assert(err, checker.IsNil)
	id := strings.TrimSpace(out)

	// Compare against expected output.
	expectedOutput := "1.2.3.4\texample.com"
	out, err = d.Cmd("exec", id, "cat", "/etc/hosts")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, expectedOutput, check.Commentf("Expected '%s', but got %q", expectedOutput, out))
}

func (s *DockerSwarmSuite) TestSwarmManagerAddress(c *check.C) {
	d1 := s.AddDaemon(c, true, true)
	d2 := s.AddDaemon(c, true, false)
	d3 := s.AddDaemon(c, true, false)

	// Manager Addresses will always show Node 1's address
	expectedOutput := fmt.Sprintf("Manager Addresses:\n  127.0.0.1:%d\n", d1.port)

	out, err := d1.Cmd("info")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, expectedOutput)

	out, err = d2.Cmd("info")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, expectedOutput)

	out, err = d3.Cmd("info")
	c.Assert(err, checker.IsNil)
	c.Assert(out, checker.Contains, expectedOutput)
}

func (s *DockerSwarmSuite) TestSwarmServiceInspectPretty(c *check.C) {
	d := s.AddDaemon(c, true, true)

	name := "top"
	out, err := d.Cmd("service", "create", "--name", name, "--limit-cpu=0.5", "busybox", "top")
	c.Assert(err, checker.IsNil, check.Commentf(out))

	expectedOutput := `
Resources:
 Limits:
  CPU:		0.5`
	out, err = d.Cmd("service", "inspect", "--pretty", name)
	c.Assert(err, checker.IsNil, check.Commentf(out))
	c.Assert(out, checker.Contains, expectedOutput, check.Commentf(out))
}
