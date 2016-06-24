package cluster

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/daemon/cluster/convert"
	executorpkg "github.com/docker/docker/daemon/cluster/executor"
	"github.com/docker/docker/daemon/cluster/executor/container"
	"github.com/docker/docker/errors"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/runconfig"
	apitypes "github.com/docker/engine-api/types"
	types "github.com/docker/engine-api/types/swarm"
	swarmagent "github.com/docker/swarmkit/agent"
	swarmapi "github.com/docker/swarmkit/api"
	"golang.org/x/net/context"
)

const swarmDirName = "swarm"
const controlSocket = "control.sock"
const swarmConnectTimeout = 20 * time.Second
const stateFile = "docker-state.json"
const defaultAddr = "0.0.0.0:2377"

const (
	initialReconnectDelay = 100 * time.Millisecond
	maxReconnectDelay     = 30 * time.Second
)

// ErrNoSwarm is returned on leaving a cluster that was never initialized
var ErrNoSwarm = fmt.Errorf("This node is not part of Swarm")

// ErrSwarmExists is returned on initialize or join request for a cluster that has already been activated
var ErrSwarmExists = fmt.Errorf("This node is already part of a Swarm cluster. Use \"docker swarm leave\" to leave this cluster and join another one.")

// ErrPendingSwarmExists is returned on initialize or join request for a cluster that is already processing a similar request but has not succeeded yet.
var ErrPendingSwarmExists = fmt.Errorf("This node is processing an existing join request that has not succeeded yet. Use \"docker swarm leave\" to cancel the current request.")

// ErrSwarmJoinTimeoutReached is returned when cluster join could not complete before timeout was reached.
var ErrSwarmJoinTimeoutReached = fmt.Errorf("Timeout was reached before node was joined. Attempt to join the cluster will continue in the background. Use \"docker info\" command to see the current Swarm status of your node.")

// defaultSpec contains some sane defaults if cluster options are missing on init
var defaultSpec = types.Spec{
	Raft: types.RaftConfig{
		SnapshotInterval:           10000,
		KeepOldSnapshots:           0,
		LogEntriesForSlowFollowers: 500,
		HeartbeatTick:              1,
		ElectionTick:               3,
	},
	CAConfig: types.CAConfig{
		NodeCertExpiry: 90 * 24 * time.Hour,
	},
	Dispatcher: types.DispatcherConfig{
		HeartbeatPeriod: uint64((5 * time.Second).Nanoseconds()),
	},
	Orchestration: types.OrchestrationConfig{
		TaskHistoryRetentionLimit: 10,
	},
}

type state struct {
	ListenAddr string
}

// Config provides values for Cluster.
type Config struct {
	Root    string
	Name    string
	Backend executorpkg.Backend
}

// Cluster provides capabilities to participate in a cluster as a worker or a
// manager.
type Cluster struct {
	sync.RWMutex
	root           string
	config         Config
	configEvent    chan struct{} // todo: make this array and goroutine safe
	node           *swarmagent.Node
	conn           *grpc.ClientConn
	client         swarmapi.ControlClient
	ready          bool
	listenAddr     string
	err            error
	reconnectDelay time.Duration
	stop           bool
	cancelDelay    func()
}

// New creates a new Cluster instance using provided config.
func New(config Config) (*Cluster, error) {
	root := filepath.Join(config.Root, swarmDirName)
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	c := &Cluster{
		root:           root,
		config:         config,
		configEvent:    make(chan struct{}, 10),
		reconnectDelay: initialReconnectDelay,
	}

	st, err := c.loadState()
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}

	n, ctx, err := c.startNewNode(false, st.ListenAddr, "", "", "", false)
	if err != nil {
		return nil, err
	}

	select {
	case <-time.After(swarmConnectTimeout):
		logrus.Errorf("swarm component could not be started before timeout was reached")
	case <-n.Ready():
	case <-ctx.Done():
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("swarm component could not be started")
	}
	go c.reconnectOnFailure(ctx)
	return c, nil
}

func (c *Cluster) loadState() (*state, error) {
	dt, err := ioutil.ReadFile(filepath.Join(c.root, stateFile))
	if err != nil {
		return nil, err
	}
	// missing certificate means no actual state to restore from
	if _, err := os.Stat(filepath.Join(c.root, "certificates/swarm-node.crt")); err != nil {
		if os.IsNotExist(err) {
			c.clearState()
		}
		return nil, err
	}
	var st state
	if err := json.Unmarshal(dt, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func (c *Cluster) saveState() error {
	dt, err := json.Marshal(state{ListenAddr: c.listenAddr})
	if err != nil {
		return err
	}
	return ioutils.AtomicWriteFile(filepath.Join(c.root, stateFile), dt, 0600)
}

func (c *Cluster) reconnectOnFailure(ctx context.Context) {
	for {
		<-ctx.Done()
		c.Lock()
		if c.stop || c.node != nil {
			c.Unlock()
			return
		}
		c.reconnectDelay *= 2
		if c.reconnectDelay > maxReconnectDelay {
			c.reconnectDelay = maxReconnectDelay
		}
		logrus.Warnf("Restarting swarm in %.2f seconds", c.reconnectDelay.Seconds())
		delayCtx, cancel := context.WithTimeout(context.Background(), c.reconnectDelay)
		c.cancelDelay = cancel
		c.Unlock()
		<-delayCtx.Done()
		if delayCtx.Err() != context.DeadlineExceeded {
			return
		}
		c.Lock()
		if c.node != nil {
			c.Unlock()
			return
		}
		var err error
		_, ctx, err = c.startNewNode(false, c.listenAddr, c.getRemoteAddress(), "", "", false)
		if err != nil {
			c.err = err
			ctx = delayCtx
		}
		c.Unlock()
	}
}

func (c *Cluster) startNewNode(forceNewCluster bool, listenAddr, joinAddr, secret, cahash string, ismanager bool) (*swarmagent.Node, context.Context, error) {
	if err := c.config.Backend.IsSwarmCompatible(); err != nil {
		return nil, nil, err
	}
	c.node = nil
	c.cancelDelay = nil
	node, err := swarmagent.NewNode(&swarmagent.NodeConfig{
		Hostname:         c.config.Name,
		ForceNewCluster:  forceNewCluster,
		ListenControlAPI: filepath.Join(c.root, controlSocket),
		ListenRemoteAPI:  listenAddr,
		JoinAddr:         joinAddr,
		StateDir:         c.root,
		CAHash:           cahash,
		Secret:           secret,
		Executor:         container.NewExecutor(c.config.Backend),
		HeartbeatTick:    1,
		ElectionTick:     3,
		IsManager:        ismanager,
	})
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := node.Start(ctx); err != nil {
		return nil, nil, err
	}

	c.node = node
	c.listenAddr = listenAddr
	c.saveState()
	c.config.Backend.SetClusterProvider(c)
	go func() {
		err := node.Err(ctx)
		if err != nil {
			logrus.Errorf("cluster exited with error: %v", err)
		}
		c.Lock()
		c.conn = nil
		c.client = nil
		c.node = nil
		c.ready = false
		c.err = err
		c.Unlock()
		cancel()
	}()

	go func() {
		select {
		case <-node.Ready():
			c.Lock()
			c.reconnectDelay = initialReconnectDelay
			c.Unlock()
		case <-ctx.Done():
		}
		if ctx.Err() == nil {
			c.Lock()
			c.ready = true
			c.err = nil
			c.Unlock()
		}
		c.configEvent <- struct{}{}
	}()

	go func() {
		for conn := range node.ListenControlSocket(ctx) {
			c.Lock()
			if c.conn != conn {
				c.client = swarmapi.NewControlClient(conn)
			}
			if c.conn != nil {
				c.client = nil
			}
			c.conn = conn
			c.Unlock()
			c.configEvent <- struct{}{}
		}
	}()

	return node, ctx, nil
}

// Init initializes new cluster from user provided request.
func (c *Cluster) Init(req types.InitRequest) (string, error) {
	c.Lock()
	if node := c.node; node != nil {
		c.Unlock()
		if !req.ForceNewCluster {
			return "", errSwarmExists(node)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		c.cancelReconnect()
		if err := c.node.Stop(ctx); err != nil && !strings.Contains(err.Error(), "context canceled") {
			return "", err
		}
		c.Lock()
		c.node = nil
		c.conn = nil
		c.ready = false
	}

	if err := validateAndSanitizeInitRequest(&req); err != nil {
		c.Unlock()
		return "", err
	}

	// todo: check current state existing
	n, ctx, err := c.startNewNode(req.ForceNewCluster, req.ListenAddr, "", "", "", false)
	if err != nil {
		c.Unlock()
		return "", err
	}
	c.Unlock()

	select {
	case <-n.Ready():
		if err := initClusterSpec(n, req.Spec); err != nil {
			return "", err
		}
		go c.reconnectOnFailure(ctx)
		return n.NodeID(), nil
	case <-ctx.Done():
		c.RLock()
		defer c.RUnlock()
		if c.err != nil {
			if !req.ForceNewCluster { // if failure on first attempt don't keep state
				if err := c.clearState(); err != nil {
					return "", err
				}
			}
			return "", c.err
		}
		return "", ctx.Err()
	}
}

// Join makes current Cluster part of an existing swarm cluster.
func (c *Cluster) Join(req types.JoinRequest) error {
	c.Lock()
	if node := c.node; node != nil {
		c.Unlock()
		return errSwarmExists(node)
	}
	if err := validateAndSanitizeJoinRequest(&req); err != nil {
		c.Unlock()
		return err
	}
	// todo: check current state existing
	n, ctx, err := c.startNewNode(false, req.ListenAddr, req.RemoteAddrs[0], req.Secret, req.CACertHash, req.Manager)
	if err != nil {
		c.Unlock()
		return err
	}
	c.Unlock()

	certificateRequested := n.CertificateRequested()
	for {
		select {
		case <-certificateRequested:
			if n.NodeMembership() == swarmapi.NodeMembershipPending {
				return fmt.Errorf("Your node is in the process of joining the cluster but needs to be accepted by existing cluster member.\nTo accept this node into cluster run \"docker node accept %v\" in an existing cluster manager. Use \"docker info\" command to see the current Swarm status of your node.", n.NodeID())
			}
			certificateRequested = nil
		case <-time.After(swarmConnectTimeout):
			// attempt to connect will continue in background, also reconnecting
			go c.reconnectOnFailure(ctx)
			return ErrSwarmJoinTimeoutReached
		case <-n.Ready():
			go c.reconnectOnFailure(ctx)
			return nil
		case <-ctx.Done():
			c.RLock()
			defer c.RUnlock()
			if c.err != nil {
				return c.err
			}
			return ctx.Err()
		}
	}
}

func (c *Cluster) cancelReconnect() {
	c.stop = true
	if c.cancelDelay != nil {
		c.cancelDelay()
		c.cancelDelay = nil
	}
}

// Leave shuts down Cluster and removes current state.
func (c *Cluster) Leave(force bool) error {
	c.Lock()
	node := c.node
	if node == nil {
		c.Unlock()
		return ErrNoSwarm
	}

	if node.Manager() != nil && !force {
		msg := "You are attempting to leave cluster on a node that is participating as a manager. "
		if c.isActiveManager() {
			active, reachable, unreachable, err := c.managerStats()
			if err == nil {
				if active && reachable-2 <= unreachable {
					if reachable == 1 && unreachable == 0 {
						msg += "Leaving last manager will remove all current state of the cluster. Use `--force` to ignore this message. "
						c.Unlock()
						return fmt.Errorf(msg)
					}
					msg += fmt.Sprintf("Leaving cluster will leave you with %v managers out of %v. This means Raft quorum will be lost and your cluster will become inaccessible. ", reachable-1, reachable+unreachable)
				}
			}
		} else {
			msg += "Doing so may lose the consensus of your cluster. "
		}

		msg += "Only way to restore a cluster that has lost consensus is to reinitialize it with `--force-new-cluster`. Use `--force` to ignore this message."
		c.Unlock()
		return fmt.Errorf(msg)
	}
	c.cancelReconnect()
	c.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := node.Stop(ctx); err != nil && !strings.Contains(err.Error(), "context canceled") {
		return err
	}
	if nodeID := node.NodeID(); nodeID != "" {
		for _, id := range c.config.Backend.ListContainersForNode(nodeID) {
			if err := c.config.Backend.ContainerRm(id, &apitypes.ContainerRmConfig{ForceRemove: true}); err != nil {
				logrus.Errorf("error removing %v: %v", id, err)
			}
		}
	}
	c.Lock()
	defer c.Unlock()
	c.node = nil
	c.conn = nil
	c.ready = false
	c.configEvent <- struct{}{}
	// todo: cleanup optional?
	if err := c.clearState(); err != nil {
		return err
	}
	return nil
}

func (c *Cluster) clearState() error {
	// todo: backup this data instead of removing?
	if err := os.RemoveAll(c.root); err != nil {
		return err
	}
	if err := os.MkdirAll(c.root, 0700); err != nil {
		return err
	}
	c.config.Backend.SetClusterProvider(nil)
	return nil
}

func (c *Cluster) getRequestContext() context.Context { // TODO: not needed when requests don't block on qourum lost
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	return ctx
}

// Inspect retrieves the configuration properties of a managed swarm cluster.
func (c *Cluster) Inspect() (types.Swarm, error) {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return types.Swarm{}, c.errNoManager()
	}

	swarm, err := getSwarm(c.getRequestContext(), c.client)
	if err != nil {
		return types.Swarm{}, err
	}

	if err != nil {
		return types.Swarm{}, err
	}

	return convert.SwarmFromGRPC(*swarm), nil
}

// Update updates configuration of a managed swarm cluster.
func (c *Cluster) Update(version uint64, spec types.Spec) error {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return c.errNoManager()
	}

	swarm, err := getSwarm(c.getRequestContext(), c.client)
	if err != nil {
		return err
	}

	swarmSpec, err := convert.SwarmSpecToGRPCandMerge(spec, &swarm.Spec)
	if err != nil {
		return err
	}

	_, err = c.client.UpdateCluster(
		c.getRequestContext(),
		&swarmapi.UpdateClusterRequest{
			ClusterID: swarm.ID,
			Spec:      &swarmSpec,
			ClusterVersion: &swarmapi.Version{
				Index: version,
			},
		},
	)
	return err
}

// IsManager returns true if Cluster is participating as a manager.
func (c *Cluster) IsManager() bool {
	c.RLock()
	defer c.RUnlock()
	return c.isActiveManager()
}

// IsAgent returns true if Cluster is participating as a worker/agent.
func (c *Cluster) IsAgent() bool {
	c.RLock()
	defer c.RUnlock()
	return c.ready
}

// GetListenAddress returns the listening address for current manager's
// consensus and dispatcher APIs.
func (c *Cluster) GetListenAddress() string {
	c.RLock()
	defer c.RUnlock()
	if c.conn != nil {
		return c.listenAddr
	}
	return ""
}

// GetRemoteAddress returns a known advertise address of a remote manager if
// available.
// todo: change to array/connect with info
func (c *Cluster) GetRemoteAddress() string {
	c.RLock()
	defer c.RUnlock()
	return c.getRemoteAddress()
}

func (c *Cluster) getRemoteAddress() string {
	if c.node == nil {
		return ""
	}
	nodeID := c.node.NodeID()
	for _, r := range c.node.Remotes() {
		if r.NodeID != nodeID {
			return r.Addr
		}
	}
	return ""
}

// ListenClusterEvents returns a channel that receives messages on cluster
// participation changes.
// todo: make cancelable and accessible to multiple callers
func (c *Cluster) ListenClusterEvents() <-chan struct{} {
	return c.configEvent
}

// Info returns information about the current cluster state.
func (c *Cluster) Info() types.Info {
	var info types.Info
	c.RLock()
	defer c.RUnlock()

	if c.node == nil {
		info.LocalNodeState = types.LocalNodeStateInactive
		if c.cancelDelay != nil {
			info.LocalNodeState = types.LocalNodeStateError
		}
	} else {
		info.LocalNodeState = types.LocalNodeStatePending
		if c.ready == true {
			info.LocalNodeState = types.LocalNodeStateActive
		}
	}
	if c.err != nil {
		info.Error = c.err.Error()
	}

	if c.isActiveManager() {
		info.ControlAvailable = true
		if r, err := c.client.ListNodes(c.getRequestContext(), &swarmapi.ListNodesRequest{}); err == nil {
			info.Nodes = len(r.Nodes)
			for _, n := range r.Nodes {
				if n.ManagerStatus != nil {
					info.Managers = info.Managers + 1
				}
			}
		}

		if swarm, err := getSwarm(c.getRequestContext(), c.client); err == nil && swarm != nil {
			info.CACertHash = swarm.RootCA.CACertHash
		}
	}

	if c.node != nil {
		for _, r := range c.node.Remotes() {
			info.RemoteManagers = append(info.RemoteManagers, types.Peer{NodeID: r.NodeID, Addr: r.Addr})
		}
		info.NodeID = c.node.NodeID()
	}

	return info
}

// isActiveManager should not be called without a read lock
func (c *Cluster) isActiveManager() bool {
	return c.conn != nil
}

// errNoManager returns error describing why manager commands can't be used.
// Call with read lock.
func (c *Cluster) errNoManager() error {
	if c.node == nil {
		return fmt.Errorf("This node is not a Swarm manager. Use \"docker swarm init\" or \"docker swarm join --manager\" to connect this node to Swarm and try again.")
	}
	if c.node.Manager() != nil {
		return fmt.Errorf("This node is not a Swarm manager. Manager is being prepared or has trouble connecting to the cluster.")
	}
	return fmt.Errorf("This node is not a Swarm manager. Worker nodes can't be used to view or modify cluster state. Please run this command on a manager node or promote the current node to a manager.")
}

// GetServices returns all services of a managed swarm cluster.
func (c *Cluster) GetServices(options apitypes.ServiceListOptions) ([]types.Service, error) {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return nil, c.errNoManager()
	}

	filters, err := newListServicesFilters(options.Filter)
	if err != nil {
		return nil, err
	}
	r, err := c.client.ListServices(
		c.getRequestContext(),
		&swarmapi.ListServicesRequest{Filters: filters})
	if err != nil {
		return nil, err
	}

	var services []types.Service

	for _, service := range r.Services {
		services = append(services, convert.ServiceFromGRPC(*service))
	}

	return services, nil
}

// CreateService creates a new service in a managed swarm cluster.
func (c *Cluster) CreateService(s types.ServiceSpec) (string, error) {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return "", c.errNoManager()
	}

	ctx := c.getRequestContext()

	err := populateNetworkID(ctx, c.client, &s)
	if err != nil {
		return "", err
	}

	serviceSpec, err := convert.ServiceSpecToGRPC(s)
	if err != nil {
		return "", err
	}
	r, err := c.client.CreateService(ctx, &swarmapi.CreateServiceRequest{Spec: &serviceSpec})
	if err != nil {
		return "", err
	}

	return r.Service.ID, nil
}

// GetService returns a service based on an ID or name.
func (c *Cluster) GetService(input string) (types.Service, error) {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return types.Service{}, c.errNoManager()
	}

	service, err := getService(c.getRequestContext(), c.client, input)
	if err != nil {
		return types.Service{}, err
	}
	return convert.ServiceFromGRPC(*service), nil
}

// UpdateService updates existing service to match new properties.
func (c *Cluster) UpdateService(serviceID string, version uint64, spec types.ServiceSpec) error {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return c.errNoManager()
	}

	serviceSpec, err := convert.ServiceSpecToGRPC(spec)
	if err != nil {
		return err
	}

	_, err = c.client.UpdateService(
		c.getRequestContext(),
		&swarmapi.UpdateServiceRequest{
			ServiceID: serviceID,
			Spec:      &serviceSpec,
			ServiceVersion: &swarmapi.Version{
				Index: version,
			},
		},
	)
	return err
}

// RemoveService removes a service from a managed swarm cluster.
func (c *Cluster) RemoveService(input string) error {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return c.errNoManager()
	}

	service, err := getService(c.getRequestContext(), c.client, input)
	if err != nil {
		return err
	}

	if _, err := c.client.RemoveService(c.getRequestContext(), &swarmapi.RemoveServiceRequest{ServiceID: service.ID}); err != nil {
		return err
	}
	return nil
}

// GetNodes returns a list of all nodes known to a cluster.
func (c *Cluster) GetNodes(options apitypes.NodeListOptions) ([]types.Node, error) {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return nil, c.errNoManager()
	}

	filters, err := newListNodesFilters(options.Filter)
	if err != nil {
		return nil, err
	}
	r, err := c.client.ListNodes(
		c.getRequestContext(),
		&swarmapi.ListNodesRequest{Filters: filters})
	if err != nil {
		return nil, err
	}

	nodes := []types.Node{}

	for _, node := range r.Nodes {
		nodes = append(nodes, convert.NodeFromGRPC(*node))
	}
	return nodes, nil
}

// GetNode returns a node based on an ID or name.
func (c *Cluster) GetNode(input string) (types.Node, error) {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return types.Node{}, c.errNoManager()
	}

	node, err := getNode(c.getRequestContext(), c.client, input)
	if err != nil {
		return types.Node{}, err
	}
	return convert.NodeFromGRPC(*node), nil
}

// UpdateNode updates existing nodes properties.
func (c *Cluster) UpdateNode(nodeID string, version uint64, spec types.NodeSpec) error {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return c.errNoManager()
	}

	nodeSpec, err := convert.NodeSpecToGRPC(spec)
	if err != nil {
		return err
	}

	_, err = c.client.UpdateNode(
		c.getRequestContext(),
		&swarmapi.UpdateNodeRequest{
			NodeID: nodeID,
			Spec:   &nodeSpec,
			NodeVersion: &swarmapi.Version{
				Index: version,
			},
		},
	)
	return err
}

// RemoveNode removes a node from a cluster
func (c *Cluster) RemoveNode(input string) error {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return c.errNoManager()
	}

	ctx := c.getRequestContext()

	node, err := getNode(ctx, c.client, input)
	if err != nil {
		return err
	}

	if _, err := c.client.RemoveNode(ctx, &swarmapi.RemoveNodeRequest{NodeID: node.ID}); err != nil {
		return err
	}
	return nil
}

// GetTasks returns a list of tasks matching the filter options.
func (c *Cluster) GetTasks(options apitypes.TaskListOptions) ([]types.Task, error) {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return nil, c.errNoManager()
	}

	filters, err := newListTasksFilters(options.Filter)
	if err != nil {
		return nil, err
	}
	r, err := c.client.ListTasks(
		c.getRequestContext(),
		&swarmapi.ListTasksRequest{Filters: filters})
	if err != nil {
		return nil, err
	}

	tasks := []types.Task{}

	for _, task := range r.Tasks {
		tasks = append(tasks, convert.TaskFromGRPC(*task))
	}
	return tasks, nil
}

// GetTask returns a task by an ID.
func (c *Cluster) GetTask(input string) (types.Task, error) {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return types.Task{}, c.errNoManager()
	}

	task, err := getTask(c.getRequestContext(), c.client, input)
	if err != nil {
		return types.Task{}, err
	}
	return convert.TaskFromGRPC(*task), nil
}

// GetNetwork returns a cluster network by an ID.
func (c *Cluster) GetNetwork(input string) (apitypes.NetworkResource, error) {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return apitypes.NetworkResource{}, c.errNoManager()
	}

	network, err := getNetwork(c.getRequestContext(), c.client, input)
	if err != nil {
		return apitypes.NetworkResource{}, err
	}
	return convert.BasicNetworkFromGRPC(*network), nil
}

// GetNetworks returns all current cluster managed networks.
func (c *Cluster) GetNetworks() ([]apitypes.NetworkResource, error) {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return nil, c.errNoManager()
	}

	r, err := c.client.ListNetworks(c.getRequestContext(), &swarmapi.ListNetworksRequest{})
	if err != nil {
		return nil, err
	}

	var networks []apitypes.NetworkResource

	for _, network := range r.Networks {
		networks = append(networks, convert.BasicNetworkFromGRPC(*network))
	}

	return networks, nil
}

// CreateNetwork creates a new cluster managed network.
func (c *Cluster) CreateNetwork(s apitypes.NetworkCreateRequest) (string, error) {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return "", c.errNoManager()
	}

	if runconfig.IsPreDefinedNetwork(s.Name) {
		err := fmt.Errorf("%s is a pre-defined network and cannot be created", s.Name)
		return "", errors.NewRequestForbiddenError(err)
	}

	networkSpec := convert.BasicNetworkCreateToGRPC(s)
	r, err := c.client.CreateNetwork(c.getRequestContext(), &swarmapi.CreateNetworkRequest{Spec: &networkSpec})
	if err != nil {
		return "", err
	}

	return r.Network.ID, nil
}

// RemoveNetwork removes a cluster network.
func (c *Cluster) RemoveNetwork(input string) error {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return c.errNoManager()
	}

	network, err := getNetwork(c.getRequestContext(), c.client, input)
	if err != nil {
		return err
	}

	if _, err := c.client.RemoveNetwork(c.getRequestContext(), &swarmapi.RemoveNetworkRequest{NetworkID: network.ID}); err != nil {
		return err
	}
	return nil
}

func populateNetworkID(ctx context.Context, c swarmapi.ControlClient, s *types.ServiceSpec) error {
	for i, n := range s.Networks {
		apiNetwork, err := getNetwork(ctx, c, n.Target)
		if err != nil {
			return err
		}
		s.Networks[i].Target = apiNetwork.ID
	}
	return nil
}

func getNetwork(ctx context.Context, c swarmapi.ControlClient, input string) (*swarmapi.Network, error) {
	// GetNetwork to match via full ID.
	rg, err := c.GetNetwork(ctx, &swarmapi.GetNetworkRequest{NetworkID: input})
	if err != nil {
		// If any error (including NotFound), ListNetworks to match via ID prefix and full name.
		rl, err := c.ListNetworks(ctx, &swarmapi.ListNetworksRequest{Filters: &swarmapi.ListNetworksRequest_Filters{Names: []string{input}}})
		if err != nil || len(rl.Networks) == 0 {
			rl, err = c.ListNetworks(ctx, &swarmapi.ListNetworksRequest{Filters: &swarmapi.ListNetworksRequest_Filters{IDPrefixes: []string{input}}})
		}

		if err != nil {
			return nil, err
		}

		if len(rl.Networks) == 0 {
			return nil, fmt.Errorf("network %s not found", input)
		}

		if l := len(rl.Networks); l > 1 {
			return nil, fmt.Errorf("network %s is ambigious (%d matches found)", input, l)
		}

		return rl.Networks[0], nil
	}
	return rg.Network, nil
}

// Cleanup stops active swarm node. This is run before daemon shutdown.
func (c *Cluster) Cleanup() {
	c.Lock()
	node := c.node
	if node == nil {
		c.Unlock()
		return
	}

	if c.isActiveManager() {
		active, reachable, unreachable, err := c.managerStats()
		if err == nil {
			singlenode := active && reachable == 1 && unreachable == 0
			if active && !singlenode && reachable-2 <= unreachable {
				logrus.Errorf("Leaving cluster with %v managers left out of %v. Raft quorum will be lost.", reachable-1, reachable+unreachable)
			}
		}
	}
	c.cancelReconnect()
	c.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := node.Stop(ctx); err != nil {
		logrus.Errorf("error cleaning up cluster: %v", err)
	}
	c.Lock()
	c.node = nil
	c.ready = false
	c.conn = nil
	c.Unlock()
}

func (c *Cluster) managerStats() (current bool, reachable int, unreachable int, err error) {
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	nodes, err := c.client.ListNodes(ctx, &swarmapi.ListNodesRequest{})
	if err != nil {
		return false, 0, 0, err
	}
	for _, n := range nodes.Nodes {
		if n.ManagerStatus != nil {
			if n.ManagerStatus.Reachability == swarmapi.RaftMemberStatus_REACHABLE {
				reachable++
				if n.ID == c.node.NodeID() {
					current = true
				}
			}
			if n.ManagerStatus.Reachability == swarmapi.RaftMemberStatus_UNREACHABLE {
				unreachable++
			}
		}
	}
	return
}

func validateAndSanitizeInitRequest(req *types.InitRequest) error {
	var err error
	req.ListenAddr, err = validateAddr(req.ListenAddr)
	if err != nil {
		return fmt.Errorf("invalid ListenAddr %q: %v", req.ListenAddr, err)
	}

	spec := &req.Spec
	// provide sane defaults instead of erroring
	if spec.Name == "" {
		spec.Name = "default"
	}
	if spec.Raft.SnapshotInterval == 0 {
		spec.Raft.SnapshotInterval = defaultSpec.Raft.SnapshotInterval
	}
	if spec.Raft.LogEntriesForSlowFollowers == 0 {
		spec.Raft.LogEntriesForSlowFollowers = defaultSpec.Raft.LogEntriesForSlowFollowers
	}
	if spec.Raft.ElectionTick == 0 {
		spec.Raft.ElectionTick = defaultSpec.Raft.ElectionTick
	}
	if spec.Raft.HeartbeatTick == 0 {
		spec.Raft.HeartbeatTick = defaultSpec.Raft.HeartbeatTick
	}
	if spec.Dispatcher.HeartbeatPeriod == 0 {
		spec.Dispatcher.HeartbeatPeriod = defaultSpec.Dispatcher.HeartbeatPeriod
	}
	if spec.CAConfig.NodeCertExpiry == 0 {
		spec.CAConfig.NodeCertExpiry = defaultSpec.CAConfig.NodeCertExpiry
	}
	if spec.Orchestration.TaskHistoryRetentionLimit == 0 {
		spec.Orchestration.TaskHistoryRetentionLimit = defaultSpec.Orchestration.TaskHistoryRetentionLimit
	}
	return nil
}

func validateAndSanitizeJoinRequest(req *types.JoinRequest) error {
	var err error
	req.ListenAddr, err = validateAddr(req.ListenAddr)
	if err != nil {
		return fmt.Errorf("invalid ListenAddr %q: %v", req.ListenAddr, err)
	}
	if len(req.RemoteAddrs) == 0 {
		return fmt.Errorf("at least 1 RemoteAddr is required to join")
	}
	for i := range req.RemoteAddrs {
		req.RemoteAddrs[i], err = validateAddr(req.RemoteAddrs[i])
		if err != nil {
			return fmt.Errorf("invalid remoteAddr %q: %v", req.RemoteAddrs[i], err)
		}
	}
	if req.CACertHash != "" {
		if _, err := digest.ParseDigest(req.CACertHash); err != nil {
			return fmt.Errorf("invalid CACertHash %q, %v", req.CACertHash, err)
		}
	}
	return nil
}

func validateAddr(addr string) (string, error) {
	if addr == "" {
		return addr, fmt.Errorf("invalid empty address")
	}
	newaddr, err := opts.ParseTCPAddr(addr, defaultAddr)
	if err != nil {
		return addr, nil
	}
	return strings.TrimPrefix(newaddr, "tcp://"), nil
}

func errSwarmExists(node *swarmagent.Node) error {
	if node.NodeMembership() != swarmapi.NodeMembershipAccepted {
		return ErrPendingSwarmExists
	}
	return ErrSwarmExists
}

func initClusterSpec(node *swarmagent.Node, spec types.Spec) error {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	for conn := range node.ListenControlSocket(ctx) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if conn != nil {
			client := swarmapi.NewControlClient(conn)
			var cluster *swarmapi.Cluster
			for i := 0; ; i++ {
				lcr, err := client.ListClusters(ctx, &swarmapi.ListClustersRequest{})
				if err != nil {
					return fmt.Errorf("error on listing clusters: %v", err)
				}
				if len(lcr.Clusters) == 0 {
					if i < 10 {
						time.Sleep(200 * time.Millisecond)
						continue
					}
					return fmt.Errorf("empty list of clusters was returned")
				}
				cluster = lcr.Clusters[0]
				break
			}
			newspec, err := convert.SwarmSpecToGRPCandMerge(spec, &cluster.Spec)
			if err != nil {
				return fmt.Errorf("error updating cluster settings: %v", err)
			}
			_, err = client.UpdateCluster(ctx, &swarmapi.UpdateClusterRequest{
				ClusterID:      cluster.ID,
				ClusterVersion: &cluster.Meta.Version,
				Spec:           &newspec,
			})
			if err != nil {
				return fmt.Errorf("error updating cluster settings: %v", err)
			}
			return nil
		}
	}
	return ctx.Err()
}
