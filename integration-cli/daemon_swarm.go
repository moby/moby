package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/swarm"
	"github.com/go-check/check"
)

// SwarmDaemon is a test daemon with helpers for participating in a swarm.
type SwarmDaemon struct {
	*Daemon
	NodeID     string
	port       int
	listenAddr string
}
type swarmInfo struct {
	IsManager bool
	IsAgent   bool
	NodeID    string
}

// Init initializes a new swarm cluster.
func (d *SwarmDaemon) Init(autoAccept map[string]bool, secret string) error {
	req := swarm.InitRequest{
		ListenAddr: d.listenAddr,
	}
	for _, role := range []string{swarm.NodeRoleManager, swarm.NodeRoleWorker} {
		req.Spec.AcceptancePolicy.Policies = append(req.Spec.AcceptancePolicy.Policies, swarm.Policy{
			Role:       role,
			Autoaccept: autoAccept[strings.ToLower(role)],
			Secret:     secret,
		})
	}
	status, out, err := d.SockRequest("POST", "/swarm/init", req)
	if status != http.StatusOK {
		return fmt.Errorf("initializing swarm: invalid statuscode %v, %q", status, out)
	}
	if err != nil {
		return fmt.Errorf("initializing swarm: %v", err)
	}
	st, err := d.swarmInfo()
	if err != nil {
		return err
	}
	d.NodeID = st.NodeID
	return nil
}

// Join joins a current daemon with existing cluster.
func (d *SwarmDaemon) Join(remoteAddr, secret string, manager bool) error {
	status, out, err := d.SockRequest("POST", "/swarm/join", swarm.JoinRequest{
		ListenAddr: d.listenAddr,
		RemoteAddr: remoteAddr,
		Manager:    manager,
		Secret:     secret,
	})
	if status != http.StatusOK {
		return fmt.Errorf("joinning swarm: invalid statuscode %v, %q", status, out)
	}
	if err != nil {
		return fmt.Errorf("joining swarm: %v", err)
	}
	st, err := d.swarmInfo()
	if err != nil {
		return err
	}
	d.NodeID = st.NodeID
	return nil
}

// Leave forces daemon to leave current cluster.
func (d *SwarmDaemon) Leave(force bool) error {
	url := "/swarm/leave"
	if force {
		url += "?force=1"
	}
	status, out, err := d.SockRequest("POST", url, nil)
	if status != http.StatusOK {
		return fmt.Errorf("leaving swarm: invalid statuscode %v, %q", status, out)
	}
	if err != nil {
		err = fmt.Errorf("leaving swarm: %v", err)
	}
	return err
}

func (d *SwarmDaemon) swarmInfo() (*swarmInfo, error) {
	status, dt, err := d.SockRequest("GET", "/info", nil)
	if status != http.StatusOK {
		return nil, fmt.Errorf("get swarm info: invalid statuscode %v", status)
	}
	if err != nil {
		return nil, fmt.Errorf("get swarm info: %v", err)
	}
	var st struct {
		Swarm swarmInfo
	}
	if err := json.Unmarshal(dt, &st); err != nil {
		return nil, err
	}
	return &st.Swarm, nil
}

// SwarmStatus returns information about the swarm manager/agent state
func (d *SwarmDaemon) SwarmStatus() (bool, bool, error) {
	st, err := d.swarmInfo()
	if err != nil {
		return false, false, err
	}
	return st.IsManager, st.IsAgent, nil
}

type serviceConstructor func(*swarm.Service)
type nodeConstructor func(*swarm.Node)

func (d *SwarmDaemon) createService(c *check.C, f ...serviceConstructor) string {
	var service swarm.Service
	for _, fn := range f {
		fn(&service)
	}
	status, out, err := d.SockRequest("POST", "/services/create", service.Spec)

	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusCreated, check.Commentf("output: %q", string(out)))

	var scr types.ServiceCreateResponse
	c.Assert(json.Unmarshal(out, &scr), checker.IsNil)
	return scr.ID
}

func (d *SwarmDaemon) getService(c *check.C, id string) *swarm.Service {
	var service swarm.Service
	status, out, err := d.SockRequest("GET", "/services/"+id, nil)
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
	c.Assert(err, checker.IsNil)
	c.Assert(json.Unmarshal(out, &service), checker.IsNil)
	c.Assert(service.ID, checker.Equals, id)
	return &service
}

func (d *SwarmDaemon) updateService(c *check.C, service *swarm.Service, f ...serviceConstructor) {
	for _, fn := range f {
		fn(service)
	}
	url := fmt.Sprintf("/services/%s/update?version=%d", service.ID, service.Version.Index)
	status, out, err := d.SockRequest("POST", url, service.Spec)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
}

func (d *SwarmDaemon) removeService(c *check.C, id string) {
	status, out, err := d.SockRequest("DELETE", "/services/"+id, nil)
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
	c.Assert(err, checker.IsNil)
}

func (d *SwarmDaemon) getNode(c *check.C, id string) *swarm.Node {
	var node swarm.Node
	status, out, err := d.SockRequest("GET", "/nodes/"+id, nil)
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
	c.Assert(err, checker.IsNil)
	c.Assert(json.Unmarshal(out, &node), checker.IsNil)
	c.Assert(node.ID, checker.Equals, id)
	return &node
}

func (d *SwarmDaemon) updateNode(c *check.C, node *swarm.Node, f ...nodeConstructor) {
	for _, fn := range f {
		fn(node)
	}
	status, out, err := d.SockRequest("POST", "/nodes/"+node.ID+"/update", node)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
}

func (d *SwarmDaemon) listNodes(c *check.C) []swarm.Node {
	status, out, err := d.SockRequest("GET", "/nodes", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))

	nodes := []swarm.Node{}
	c.Assert(json.Unmarshal(out, &nodes), checker.IsNil)
	return nodes
}
