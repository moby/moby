package daemon // import "github.com/docker/docker/integration-cli/daemon"

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/go-check/check"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// Swarm is a test daemon with helpers for participating in a swarm.
type Swarm struct {
	*Daemon
	swarm.Info
	Port       int
	ListenAddr string
}

// Init initializes a new swarm cluster.
func (d *Swarm) Init(req swarm.InitRequest) error {
	if req.ListenAddr == "" {
		req.ListenAddr = d.ListenAddr
	}
	cli, err := d.NewClient()
	if err != nil {
		return fmt.Errorf("initializing swarm: failed to create client %v", err)
	}
	defer cli.Close()
	_, err = cli.SwarmInit(context.Background(), req)
	if err != nil {
		return fmt.Errorf("initializing swarm: %v", err)
	}
	info, err := d.SwarmInfo()
	if err != nil {
		return err
	}
	d.Info = info
	return nil
}

// Join joins a daemon to an existing cluster.
func (d *Swarm) Join(req swarm.JoinRequest) error {
	if req.ListenAddr == "" {
		req.ListenAddr = d.ListenAddr
	}
	cli, err := d.NewClient()
	if err != nil {
		return fmt.Errorf("joining swarm: failed to create client %v", err)
	}
	defer cli.Close()
	err = cli.SwarmJoin(context.Background(), req)
	if err != nil {
		return fmt.Errorf("joining swarm: %v", err)
	}
	info, err := d.SwarmInfo()
	if err != nil {
		return err
	}
	d.Info = info
	return nil
}

// Leave forces daemon to leave current cluster.
func (d *Swarm) Leave(force bool) error {
	cli, err := d.NewClient()
	if err != nil {
		return fmt.Errorf("leaving swarm: failed to create client %v", err)
	}
	defer cli.Close()
	err = cli.SwarmLeave(context.Background(), force)
	if err != nil {
		err = fmt.Errorf("leaving swarm: %v", err)
	}
	return err
}

// SwarmInfo returns the swarm information of the daemon
func (d *Swarm) SwarmInfo() (swarm.Info, error) {
	cli, err := d.NewClient()
	if err != nil {
		return swarm.Info{}, fmt.Errorf("get swarm info: %v", err)
	}

	info, err := cli.Info(context.Background())
	if err != nil {
		return swarm.Info{}, fmt.Errorf("get swarm info: %v", err)
	}

	return info.Swarm, nil
}

// Unlock tries to unlock a locked swarm
func (d *Swarm) Unlock(req swarm.UnlockRequest) error {
	cli, err := d.NewClient()
	if err != nil {
		return fmt.Errorf("unlocking swarm: failed to create client %v", err)
	}
	defer cli.Close()
	err = cli.SwarmUnlock(context.Background(), req)
	if err != nil {
		err = errors.Wrap(err, "unlocking swarm")
	}
	return err
}

// ServiceConstructor defines a swarm service constructor function
type ServiceConstructor func(*swarm.Service)

// NodeConstructor defines a swarm node constructor
type NodeConstructor func(*swarm.Node)

// SecretConstructor defines a swarm secret constructor
type SecretConstructor func(*swarm.Secret)

// ConfigConstructor defines a swarm config constructor
type ConfigConstructor func(*swarm.Config)

// SpecConstructor defines a swarm spec constructor
type SpecConstructor func(*swarm.Spec)

// CreateServiceWithOptions creates a swarm service given the specified service constructors
// and auth config
func (d *Swarm) CreateServiceWithOptions(c *check.C, opts types.ServiceCreateOptions, f ...ServiceConstructor) string {
	var service swarm.Service
	for _, fn := range f {
		fn(&service)
	}

	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := cli.ServiceCreate(ctx, service.Spec, opts)
	c.Assert(err, checker.IsNil)
	return res.ID
}

// CreateService creates a swarm service given the specified service constructor
func (d *Swarm) CreateService(c *check.C, f ...ServiceConstructor) string {
	return d.CreateServiceWithOptions(c, types.ServiceCreateOptions{}, f...)
}

// GetService returns the swarm service corresponding to the specified id
func (d *Swarm) GetService(c *check.C, id string) *swarm.Service {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	service, _, err := cli.ServiceInspectWithRaw(context.Background(), id, types.ServiceInspectOptions{})
	c.Assert(err, checker.IsNil)
	return &service
}

// GetServiceTasks returns the swarm tasks for the specified service
func (d *Swarm) GetServiceTasks(c *check.C, service string) []swarm.Task {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	filterArgs := filters.NewArgs()
	filterArgs.Add("desired-state", "running")
	filterArgs.Add("service", service)

	options := types.TaskListOptions{
		Filters: filterArgs,
	}

	tasks, err := cli.TaskList(context.Background(), options)
	c.Assert(err, checker.IsNil)
	return tasks
}

// CheckServiceTasksInState returns the number of tasks with a matching state,
// and optional message substring.
func (d *Swarm) CheckServiceTasksInState(service string, state swarm.TaskState, message string) func(*check.C) (interface{}, check.CommentInterface) {
	return func(c *check.C) (interface{}, check.CommentInterface) {
		tasks := d.GetServiceTasks(c, service)
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

// CheckServiceTasksInStateWithError returns the number of tasks with a matching state,
// and optional message substring.
func (d *Swarm) CheckServiceTasksInStateWithError(service string, state swarm.TaskState, errorMessage string) func(*check.C) (interface{}, check.CommentInterface) {
	return func(c *check.C) (interface{}, check.CommentInterface) {
		tasks := d.GetServiceTasks(c, service)
		var count int
		for _, task := range tasks {
			if task.Status.State == state {
				if errorMessage == "" || strings.Contains(task.Status.Err, errorMessage) {
					count++
				}
			}
		}
		return count, nil
	}
}

// CheckServiceRunningTasks returns the number of running tasks for the specified service
func (d *Swarm) CheckServiceRunningTasks(service string) func(*check.C) (interface{}, check.CommentInterface) {
	return d.CheckServiceTasksInState(service, swarm.TaskStateRunning, "")
}

// CheckServiceUpdateState returns the current update state for the specified service
func (d *Swarm) CheckServiceUpdateState(service string) func(*check.C) (interface{}, check.CommentInterface) {
	return func(c *check.C) (interface{}, check.CommentInterface) {
		service := d.GetService(c, service)
		if service.UpdateStatus == nil {
			return "", nil
		}
		return service.UpdateStatus.State, nil
	}
}

// CheckPluginRunning returns the runtime state of the plugin
func (d *Swarm) CheckPluginRunning(plugin string) func(c *check.C) (interface{}, check.CommentInterface) {
	return func(c *check.C) (interface{}, check.CommentInterface) {
		status, out, err := d.SockRequest("GET", "/plugins/"+plugin+"/json", nil)
		c.Assert(err, checker.IsNil, check.Commentf(string(out)))
		if status != http.StatusOK {
			return false, nil
		}

		var p types.Plugin
		c.Assert(json.Unmarshal(out, &p), checker.IsNil, check.Commentf(string(out)))

		return p.Enabled, check.Commentf("%+v", p)
	}
}

// CheckPluginImage returns the runtime state of the plugin
func (d *Swarm) CheckPluginImage(plugin string) func(c *check.C) (interface{}, check.CommentInterface) {
	return func(c *check.C) (interface{}, check.CommentInterface) {
		status, out, err := d.SockRequest("GET", "/plugins/"+plugin+"/json", nil)
		c.Assert(err, checker.IsNil, check.Commentf(string(out)))
		if status != http.StatusOK {
			return false, nil
		}

		var p types.Plugin
		c.Assert(json.Unmarshal(out, &p), checker.IsNil, check.Commentf(string(out)))
		return p.PluginReference, check.Commentf("%+v", p)
	}
}

// CheckServiceTasks returns the number of tasks for the specified service
func (d *Swarm) CheckServiceTasks(service string) func(*check.C) (interface{}, check.CommentInterface) {
	return func(c *check.C) (interface{}, check.CommentInterface) {
		tasks := d.GetServiceTasks(c, service)
		return len(tasks), nil
	}
}

// CheckRunningTaskNetworks returns the number of times each network is referenced from a task.
func (d *Swarm) CheckRunningTaskNetworks(c *check.C) (interface{}, check.CommentInterface) {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	filterArgs := filters.NewArgs()
	filterArgs.Add("desired-state", "running")

	options := types.TaskListOptions{
		Filters: filterArgs,
	}

	tasks, err := cli.TaskList(context.Background(), options)
	c.Assert(err, checker.IsNil)

	result := make(map[string]int)
	for _, task := range tasks {
		for _, network := range task.Spec.Networks {
			result[network.Target]++
		}
	}
	return result, nil
}

// CheckRunningTaskImages returns the times each image is running as a task.
func (d *Swarm) CheckRunningTaskImages(c *check.C) (interface{}, check.CommentInterface) {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	filterArgs := filters.NewArgs()
	filterArgs.Add("desired-state", "running")

	options := types.TaskListOptions{
		Filters: filterArgs,
	}

	tasks, err := cli.TaskList(context.Background(), options)
	c.Assert(err, checker.IsNil)

	result := make(map[string]int)
	for _, task := range tasks {
		if task.Status.State == swarm.TaskStateRunning && task.Spec.ContainerSpec != nil {
			result[task.Spec.ContainerSpec.Image]++
		}
	}
	return result, nil
}

// CheckNodeReadyCount returns the number of ready node on the swarm
func (d *Swarm) CheckNodeReadyCount(c *check.C) (interface{}, check.CommentInterface) {
	nodes := d.ListNodes(c)
	var readyCount int
	for _, node := range nodes {
		if node.Status.State == swarm.NodeStateReady {
			readyCount++
		}
	}
	return readyCount, nil
}

// GetTask returns the swarm task identified by the specified id
func (d *Swarm) GetTask(c *check.C, id string) swarm.Task {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	task, _, err := cli.TaskInspectWithRaw(context.Background(), id)
	c.Assert(err, checker.IsNil)
	return task
}

// UpdateService updates a swarm service with the specified service constructor
func (d *Swarm) UpdateService(c *check.C, service *swarm.Service, f ...ServiceConstructor) {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	for _, fn := range f {
		fn(service)
	}

	_, err = cli.ServiceUpdate(context.Background(), service.ID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	c.Assert(err, checker.IsNil)
}

// RemoveService removes the specified service
func (d *Swarm) RemoveService(c *check.C, id string) {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	err = cli.ServiceRemove(context.Background(), id)
	c.Assert(err, checker.IsNil)
}

// GetNode returns a swarm node identified by the specified id
func (d *Swarm) GetNode(c *check.C, id string) *swarm.Node {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	node, _, err := cli.NodeInspectWithRaw(context.Background(), id)
	c.Assert(err, checker.IsNil)
	c.Assert(node.ID, checker.Equals, id)
	return &node
}

// RemoveNode removes the specified node
func (d *Swarm) RemoveNode(c *check.C, id string, force bool) {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	options := types.NodeRemoveOptions{
		Force: force,
	}
	err = cli.NodeRemove(context.Background(), id, options)
	c.Assert(err, checker.IsNil)
}

// UpdateNode updates a swarm node with the specified node constructor
func (d *Swarm) UpdateNode(c *check.C, id string, f ...NodeConstructor) {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	for i := 0; ; i++ {
		node := d.GetNode(c, id)
		for _, fn := range f {
			fn(node)
		}

		err = cli.NodeUpdate(context.Background(), node.ID, node.Version, node.Spec)
		if i < 10 && err != nil && strings.Contains(err.Error(), "update out of sequence") {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		c.Assert(err, checker.IsNil)
		return
	}
}

// ListNodes returns the list of the current swarm nodes
func (d *Swarm) ListNodes(c *check.C) []swarm.Node {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	nodes, err := cli.NodeList(context.Background(), types.NodeListOptions{})
	c.Assert(err, checker.IsNil)

	return nodes
}

// ListServices returns the list of the current swarm services
func (d *Swarm) ListServices(c *check.C) []swarm.Service {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	services, err := cli.ServiceList(context.Background(), types.ServiceListOptions{})
	c.Assert(err, checker.IsNil)
	return services
}

// CreateSecret creates a secret given the specified spec
func (d *Swarm) CreateSecret(c *check.C, secretSpec swarm.SecretSpec) string {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	scr, err := cli.SecretCreate(context.Background(), secretSpec)
	c.Assert(err, checker.IsNil)

	return scr.ID
}

// ListSecrets returns the list of the current swarm secrets
func (d *Swarm) ListSecrets(c *check.C) []swarm.Secret {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	secrets, err := cli.SecretList(context.Background(), types.SecretListOptions{})
	c.Assert(err, checker.IsNil)
	return secrets
}

// GetSecret returns a swarm secret identified by the specified id
func (d *Swarm) GetSecret(c *check.C, id string) *swarm.Secret {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	secret, _, err := cli.SecretInspectWithRaw(context.Background(), id)
	c.Assert(err, checker.IsNil)
	return &secret
}

// DeleteSecret removes the swarm secret identified by the specified id
func (d *Swarm) DeleteSecret(c *check.C, id string) {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	err = cli.SecretRemove(context.Background(), id)
	c.Assert(err, checker.IsNil)
}

// UpdateSecret updates the swarm secret identified by the specified id
// Currently, only label update is supported.
func (d *Swarm) UpdateSecret(c *check.C, id string, f ...SecretConstructor) {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	secret := d.GetSecret(c, id)
	for _, fn := range f {
		fn(secret)
	}

	err = cli.SecretUpdate(context.Background(), secret.ID, secret.Version, secret.Spec)

	c.Assert(err, checker.IsNil)
}

// CreateConfig creates a config given the specified spec
func (d *Swarm) CreateConfig(c *check.C, configSpec swarm.ConfigSpec) string {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	scr, err := cli.ConfigCreate(context.Background(), configSpec)
	c.Assert(err, checker.IsNil)
	return scr.ID
}

// ListConfigs returns the list of the current swarm configs
func (d *Swarm) ListConfigs(c *check.C) []swarm.Config {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	configs, err := cli.ConfigList(context.Background(), types.ConfigListOptions{})
	c.Assert(err, checker.IsNil)
	return configs
}

// GetConfig returns a swarm config identified by the specified id
func (d *Swarm) GetConfig(c *check.C, id string) *swarm.Config {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	config, _, err := cli.ConfigInspectWithRaw(context.Background(), id)
	c.Assert(err, checker.IsNil)
	return &config
}

// DeleteConfig removes the swarm config identified by the specified id
func (d *Swarm) DeleteConfig(c *check.C, id string) {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	err = cli.ConfigRemove(context.Background(), id)
	c.Assert(err, checker.IsNil)
}

// UpdateConfig updates the swarm config identified by the specified id
// Currently, only label update is supported.
func (d *Swarm) UpdateConfig(c *check.C, id string, f ...ConfigConstructor) {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	config := d.GetConfig(c, id)
	for _, fn := range f {
		fn(config)
	}

	err = cli.ConfigUpdate(context.Background(), config.ID, config.Version, config.Spec)
	c.Assert(err, checker.IsNil)
}

// GetSwarm returns the current swarm object
func (d *Swarm) GetSwarm(c *check.C) swarm.Swarm {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	sw, err := cli.SwarmInspect(context.Background())
	c.Assert(err, checker.IsNil)
	return sw
}

// UpdateSwarm updates the current swarm object with the specified spec constructors
func (d *Swarm) UpdateSwarm(c *check.C, f ...SpecConstructor) {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	sw := d.GetSwarm(c)
	for _, fn := range f {
		fn(&sw.Spec)
	}

	err = cli.SwarmUpdate(context.Background(), sw.Version, sw.Spec, swarm.UpdateFlags{})
	c.Assert(err, checker.IsNil)
}

// RotateTokens update the swarm to rotate tokens
func (d *Swarm) RotateTokens(c *check.C) {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	sw, err := cli.SwarmInspect(context.Background())
	c.Assert(err, checker.IsNil)

	flags := swarm.UpdateFlags{
		RotateManagerToken: true,
		RotateWorkerToken:  true,
	}

	err = cli.SwarmUpdate(context.Background(), sw.Version, sw.Spec, flags)
	c.Assert(err, checker.IsNil)
}

// JoinTokens returns the current swarm join tokens
func (d *Swarm) JoinTokens(c *check.C) swarm.JoinTokens {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	sw, err := cli.SwarmInspect(context.Background())
	c.Assert(err, checker.IsNil)
	return sw.JoinTokens
}

// CheckLocalNodeState returns the current swarm node state
func (d *Swarm) CheckLocalNodeState(c *check.C) (interface{}, check.CommentInterface) {
	info, err := d.SwarmInfo()
	c.Assert(err, checker.IsNil)
	return info.LocalNodeState, nil
}

// CheckControlAvailable returns the current swarm control available
func (d *Swarm) CheckControlAvailable(c *check.C) (interface{}, check.CommentInterface) {
	info, err := d.SwarmInfo()
	c.Assert(err, checker.IsNil)
	c.Assert(info.LocalNodeState, checker.Equals, swarm.LocalNodeStateActive)
	return info.ControlAvailable, nil
}

// CheckLeader returns whether there is a leader on the swarm or not
func (d *Swarm) CheckLeader(c *check.C) (interface{}, check.CommentInterface) {
	cli, err := d.NewClient()
	c.Assert(err, checker.IsNil)
	defer cli.Close()

	errList := check.Commentf("could not get node list")

	ls, err := cli.NodeList(context.Background(), types.NodeListOptions{})
	if err != nil {
		return err, errList
	}

	for _, node := range ls {
		if node.ManagerStatus != nil && node.ManagerStatus.Leader {
			return nil, nil
		}
	}
	return fmt.Errorf("no leader"), check.Commentf("could not find leader")
}

// CmdRetryOutOfSequence tries the specified command against the current daemon for 10 times
func (d *Swarm) CmdRetryOutOfSequence(args ...string) (string, error) {
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
