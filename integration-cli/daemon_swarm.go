package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/integration/checker"
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
func (d *SwarmDaemon) Init(req swarm.InitRequest) error {
	if req.ListenAddr == "" {
		req.ListenAddr = d.listenAddr
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

// Join joins a daemon to an existing cluster.
func (d *SwarmDaemon) Join(req swarm.JoinRequest) error {
	if req.ListenAddr == "" {
		req.ListenAddr = d.listenAddr
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

	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusCreated, check.Commentf("output: %q", string(out)))

	var scr types.ServiceCreateResponse
	c.Assert(json.Unmarshal(out, &scr), checker.IsNil)
	return scr.ID
}

func (d *SwarmDaemon) getService(c *check.C, id string) *swarm.Service {
	var service swarm.Service
	status, out, err := d.SockRequest("GET", "/services/"+id, nil)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
	c.Assert(json.Unmarshal(out, &service), checker.IsNil)
	return &service
}

func (d *SwarmDaemon) getServiceTasks(c *check.C, service string) []swarm.Task {
	var tasks []swarm.Task

	filterArgs := filters.NewArgs()
	filterArgs.Add("desired-state", "running")
	filterArgs.Add("service", service)
	filters, err := filters.ToParam(filterArgs)
	c.Assert(err, checker.IsNil)

	status, out, err := d.SockRequest("GET", "/tasks?filters="+filters, nil)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
	c.Assert(json.Unmarshal(out, &tasks), checker.IsNil)
	return tasks
}

func (d *SwarmDaemon) checkServiceTasksInState(service string, state swarm.TaskState, message string) func(*check.C) (interface{}, check.CommentInterface) {
	return func(c *check.C) (interface{}, check.CommentInterface) {
		tasks := d.getServiceTasks(c, service)
		var count int
		for _, task := range tasks {
			if task.Status.State == state {
				if message == "" || strings.Contains(task.Status.Message, message) {
					count++
				}
			}
		}
		return count, nil
	}
}

func (d *SwarmDaemon) checkServiceRunningTasks(service string) func(*check.C) (interface{}, check.CommentInterface) {
	return d.checkServiceTasksInState(service, swarm.TaskStateRunning, "")
}

func (d *SwarmDaemon) checkServiceUpdateState(service string) func(*check.C) (interface{}, check.CommentInterface) {
	return func(c *check.C) (interface{}, check.CommentInterface) {
		service := d.getService(c, service)
		return service.UpdateStatus.State, nil
	}
}

func (d *SwarmDaemon) checkServiceTasks(service string) func(*check.C) (interface{}, check.CommentInterface) {
	return func(c *check.C) (interface{}, check.CommentInterface) {
		tasks := d.getServiceTasks(c, service)
		return len(tasks), nil
	}
}

func (d *SwarmDaemon) checkRunningTaskImages(c *check.C) (interface{}, check.CommentInterface) {
	var tasks []swarm.Task

	filterArgs := filters.NewArgs()
	filterArgs.Add("desired-state", "running")
	filters, err := filters.ToParam(filterArgs)
	c.Assert(err, checker.IsNil)

	status, out, err := d.SockRequest("GET", "/tasks?filters="+filters, nil)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
	c.Assert(json.Unmarshal(out, &tasks), checker.IsNil)

	result := make(map[string]int)
	for _, task := range tasks {
		if task.Status.State == swarm.TaskStateRunning {
			result[task.Spec.ContainerSpec.Image]++
		}
	}
	return result, nil
}

func (d *SwarmDaemon) checkNodeReadyCount(c *check.C) (interface{}, check.CommentInterface) {
	nodes := d.listNodes(c)
	var readyCount int
	for _, node := range nodes {
		if node.Status.State == swarm.NodeStateReady {
			readyCount++
		}
	}
	return readyCount, nil
}

func (d *SwarmDaemon) getTask(c *check.C, id string) swarm.Task {
	var task swarm.Task

	status, out, err := d.SockRequest("GET", "/tasks/"+id, nil)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
	c.Assert(json.Unmarshal(out, &task), checker.IsNil)
	return task
}

func (d *SwarmDaemon) updateService(c *check.C, service *swarm.Service, f ...serviceConstructor) {
	for _, fn := range f {
		fn(service)
	}
	url := fmt.Sprintf("/services/%s/update?version=%d", service.ID, service.Version.Index)
	status, out, err := d.SockRequest("POST", url, service.Spec)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
}

func (d *SwarmDaemon) removeService(c *check.C, id string) {
	status, out, err := d.SockRequest("DELETE", "/services/"+id, nil)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
}

func (d *SwarmDaemon) getNode(c *check.C, id string) *swarm.Node {
	var node swarm.Node
	status, out, err := d.SockRequest("GET", "/nodes/"+id, nil)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
	c.Assert(json.Unmarshal(out, &node), checker.IsNil)
	c.Assert(node.ID, checker.Equals, id)
	return &node
}

func (d *SwarmDaemon) removeNode(c *check.C, id string, force bool) {
	url := "/nodes/" + id
	if force {
		url += "?force=1"
	}

	status, out, err := d.SockRequest("DELETE", url, nil)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
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
		c.Assert(err, checker.IsNil, check.Commentf(string(out)))
		c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
		return
	}
}

func (d *SwarmDaemon) listNodes(c *check.C) []swarm.Node {
	status, out, err := d.SockRequest("GET", "/nodes", nil)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))

	nodes := []swarm.Node{}
	c.Assert(json.Unmarshal(out, &nodes), checker.IsNil)
	return nodes
}

func (d *SwarmDaemon) listServices(c *check.C) []swarm.Service {
	status, out, err := d.SockRequest("GET", "/services", nil)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))

	services := []swarm.Service{}
	c.Assert(json.Unmarshal(out, &services), checker.IsNil)
	return services
}

func (d *SwarmDaemon) createSecret(c *check.C, secretSpec swarm.SecretSpec) string {
	status, out, err := d.SockRequest("POST", "/secrets/create", secretSpec)

	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusCreated, check.Commentf("output: %q", string(out)))

	var scr types.SecretCreateResponse
	c.Assert(json.Unmarshal(out, &scr), checker.IsNil)
	return scr.ID
}

func (d *SwarmDaemon) listSecrets(c *check.C) []swarm.Secret {
	status, out, err := d.SockRequest("GET", "/secrets", nil)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))

	secrets := []swarm.Secret{}
	c.Assert(json.Unmarshal(out, &secrets), checker.IsNil)
	return secrets
}

func (d *SwarmDaemon) getSecret(c *check.C, id string) *swarm.Secret {
	var secret swarm.Secret
	status, out, err := d.SockRequest("GET", "/secrets/"+id, nil)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
	c.Assert(json.Unmarshal(out, &secret), checker.IsNil)
	return &secret
}

func (d *SwarmDaemon) deleteSecret(c *check.C, id string) {
	status, out, err := d.SockRequest("DELETE", "/secrets/"+id, nil)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusNoContent, check.Commentf("output: %q", string(out)))
}

func (d *SwarmDaemon) getSwarm(c *check.C) swarm.Swarm {
	var sw swarm.Swarm
	status, out, err := d.SockRequest("GET", "/swarm", nil)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
	c.Assert(json.Unmarshal(out, &sw), checker.IsNil)
	return sw
}

func (d *SwarmDaemon) updateSwarm(c *check.C, f ...specConstructor) {
	sw := d.getSwarm(c)
	for _, fn := range f {
		fn(&sw.Spec)
	}
	url := fmt.Sprintf("/swarm/update?version=%d", sw.Version.Index)
	status, out, err := d.SockRequest("POST", url, sw.Spec)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
}

func (d *SwarmDaemon) rotateTokens(c *check.C) {
	var sw swarm.Swarm
	status, out, err := d.SockRequest("GET", "/swarm", nil)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
	c.Assert(json.Unmarshal(out, &sw), checker.IsNil)

	url := fmt.Sprintf("/swarm/update?version=%d&rotateWorkerToken=true&rotateManagerToken=true", sw.Version.Index)
	status, out, err = d.SockRequest("POST", url, sw.Spec)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
}

func (d *SwarmDaemon) joinTokens(c *check.C) swarm.JoinTokens {
	var sw swarm.Swarm
	status, out, err := d.SockRequest("GET", "/swarm", nil)
	c.Assert(err, checker.IsNil, check.Commentf(string(out)))
	c.Assert(status, checker.Equals, http.StatusOK, check.Commentf("output: %q", string(out)))
	c.Assert(json.Unmarshal(out, &sw), checker.IsNil)
	return sw.JoinTokens
}

func (d *SwarmDaemon) checkLocalNodeState(c *check.C) (interface{}, check.CommentInterface) {
	info, err := d.info()
	c.Assert(err, checker.IsNil)
	return info.LocalNodeState, nil
}

func (d *SwarmDaemon) checkControlAvailable(c *check.C) (interface{}, check.CommentInterface) {
	info, err := d.info()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
	return info.ControlAvailable, nil
}

func (d *SwarmDaemon) checkLeader(c *check.C) (interface{}, check.CommentInterface) {
	errList := check.Commentf("could not get node list")
	status, out, err := d.SockRequest("GET", "/nodes", nil)
	if err != nil {
		return err, errList
	}
	if status != http.StatusOK {
		return fmt.Errorf("expected http status OK, got: %d", status), errList
	}

	var ls []swarm.Node
	if err := json.Unmarshal(out, &ls); err != nil {
		return err, errList
	}

	for _, node := range ls {
		if node.ManagerStatus != nil && node.ManagerStatus.Leader {
			return nil, nil
		}
	}
	return fmt.Errorf("no leader"), check.Commentf("could not find leader")
}

func (d *SwarmDaemon) cmdRetryOutOfSequence(args ...string) (string, error) {
	for i := 0; ; i++ {
		out, err := d.Cmd(args...)
		if err != nil {
			if strings.Contains(out, "update out of sequence") {
				if i < 10 {
					continue
				}
			}
		}
		return out, err
	}
}
