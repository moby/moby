package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/swarm"
	"github.com/go-check/check"
)

// SwarmDaemon is a test daemon with helpers for participating in a swarm.
type SwarmDaemon struct {
	*Daemon
	swarm.Info
	port       int
	listenAddr string
}

// Init initializes a new swarm cluster.
func (d *SwarmDaemon) Init(autoAccept map[string]bool, secret string) error {
	req := swarm.InitRequest{
		ListenAddr: d.listenAddr,
	}
	for _, role := range []swarm.NodeRole{swarm.NodeRoleManager, swarm.NodeRoleWorker} {
		policy := swarm.Policy{
			Role:       role,
			Autoaccept: autoAccept[strings.ToLower(string(role))],
		}

		if secret != "" {
			policy.Secret = &secret
		}

		req.Spec.AcceptancePolicy.Policies = append(req.Spec.AcceptancePolicy.Policies, policy)
	}
	status, out, err := d.SockRequest("POST", "/swarm/init", req)
	if status != http.StatusOK {
		return fmt.Errorf("initializing swarm: invalid statuscode %v, %q", status, out)
	}
	if err != nil {
		return fmt.Errorf("initializing swarm: %v", err)
	}
	info, err := d.info()
	if err != nil {
		return err
	}
	d.Info = info
	return nil
}

// Join joins a current daemon with existing cluster.
func (d *SwarmDaemon) Join(remoteAddr, secret, cahash string, manager bool) error {
	req := swarm.JoinRequest{
		ListenAddr:  d.listenAddr,
		RemoteAddrs: []string{remoteAddr},
		Manager:     manager,
		CACertHash:  cahash,
	}

	if secret != "" {
		req.Secret = secret
	}
	status, out, err := d.SockRequest("POST", "/swarm/join", req)
	if status != http.StatusOK {
		return fmt.Errorf("joining swarm: invalid statuscode %v, %q", status, out)
	}
	if err != nil {
		return fmt.Errorf("joining swarm: %v", err)
	}
	info, err := d.info()
	if err != nil {
		return err
	}
	d.Info = info
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

func (d *SwarmDaemon) info() (swarm.Info, error) {
	var info struct {
		Swarm swarm.Info
	}
	status, dt, err := d.SockRequest("GET", "/info", nil)
	if status != http.StatusOK {
		return info.Swarm, fmt.Errorf("get swarm info: invalid statuscode %v", status)
	}
	if err != nil {
		return info.Swarm, fmt.Errorf("get swarm info: %v", err)
	}
	if err := json.Unmarshal(dt, &info); err != nil {
		return info.Swarm, err
	}
	return info.Swarm, nil
}

type serviceConstructor func(*swarm.Service)
type nodeConstructor func(*swarm.Node)
type specConstructor func(*swarm.Spec)

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

func (d *SwarmDaemon) updateNode(c *check.C, id string, f ...nodeConstructor) {
	for i := 0; ; i++ {
		node := d.getNode(c, id)
		for _, fn := range f {
			fn(node)
		}
		url := fmt.Sprintf("/nodes/%s/update?version=%d", node.ID, node.Version.Index)
		status, out, err := d.SockRequest("POST", url, node.Spec)
		if i < 10 && strings.Contains(string(out), "update out of sequence") {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		c.Assert(err, checker.IsNil)
		c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
		return
	}
}

func (d *SwarmDaemon) listNodes(c *check.C) []swarm.Node {
	status, out, err := d.SockRequest("GET", "/nodes", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))

	nodes := []swarm.Node{}
	c.Assert(json.Unmarshal(out, &nodes), checker.IsNil)
	return nodes
}

func (d *SwarmDaemon) updateSwarm(c *check.C, f ...specConstructor) {
	var sw swarm.Swarm
	status, out, err := d.SockRequest("GET", "/swarm", nil)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
	c.Assert(json.Unmarshal(out, &sw), checker.IsNil)

	for _, fn := range f {
		fn(&sw.Spec)
	}
	url := fmt.Sprintf("/swarm/update?version=%d", sw.Version.Index)
	status, out, err = d.SockRequest("POST", url, sw.Spec)
	c.Assert(err, checker.IsNil)
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
}
