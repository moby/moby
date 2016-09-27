package cluster

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/errors"
	apitypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	types "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/daemon/cluster/convert"
	executorpkg "github.com/docker/docker/daemon/cluster/executor"
	"github.com/docker/docker/daemon/cluster/executor/container"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/runconfig"
	swarmagent "github.com/docker/swarmkit/agent"
	swarmapi "github.com/docker/swarmkit/api"
	"golang.org/x/net/context"
)

const swarmDirName = "swarm"
const controlSocket = "control.sock"
const swarmConnectTimeout = 20 * time.Second
const swarmRequestTimeout = 20 * time.Second
const stateFile = "docker-state.json"
const defaultAddr = "0.0.0.0:2377"

const (
	initialReconnectDelay = 100 * time.Millisecond
	maxReconnectDelay     = 30 * time.Second
)

// ErrNoSwarm is returned on leaving a cluster that was never initialized
var ErrNoSwarm = fmt.Errorf("This node is not part of a swarm")

// ErrSwarmExists is returned on initialize or join request for a cluster that has already been activated
var ErrSwarmExists = fmt.Errorf("This node is already part of a swarm. Use \"docker swarm leave\" to leave this swarm and join another one.")

// ErrPendingSwarmExists is returned on initialize or join request for a cluster that is already processing a similar request but has not succeeded yet.
var ErrPendingSwarmExists = fmt.Errorf("This node is processing an existing join request that has not succeeded yet. Use \"docker swarm leave\" to cancel the current request.")

// ErrSwarmJoinTimeoutReached is returned when cluster join could not complete before timeout was reached.
var ErrSwarmJoinTimeoutReached = fmt.Errorf("Timeout was reached before node was joined. The attempt to join the swarm will continue in the background. Use the \"docker info\" command to see the current swarm status of your node.")

type state struct {
	// LocalAddr is this machine's local IP or hostname, if specified.
	LocalAddr string
	// RemoteAddr is the address that was given to "swarm join. It is used
	// to find LocalAddr if necessary.
	RemoteAddr string
	// ListenAddr is the address we bind to, including a port.
	ListenAddr string
	// AdvertiseAddr is the address other nodes should connect to,
	// including a port.
	AdvertiseAddr string
}

// NetworkSubnetsProvider exposes functions for retrieving the subnets
// of networks managed by Docker, so they can be filtered.
type NetworkSubnetsProvider interface {
	V4Subnets() []net.IPNet
	V6Subnets() []net.IPNet
}

// Config provides values for Cluster.
type Config struct {
	Root                   string
	Name                   string
	Backend                executorpkg.Backend
	NetworkSubnetsProvider NetworkSubnetsProvider

	// DefaultAdvertiseAddr is the default host/IP or network interface to use
	// if no AdvertiseAddr value is specified.
	DefaultAdvertiseAddr string

	// path to store runtime state, such as the swarm control socket
	RuntimeRoot string
}

// Cluster provides capabilities to participate in a cluster as a worker or a
// manager.
type Cluster struct {
	sync.RWMutex
	*node
	root            string
	runtimeRoot     string
	config          Config
	configEvent     chan struct{} // todo: make this array and goroutine safe
	localAddr       string
	actualLocalAddr string // after resolution, not persisted
	remoteAddr      string
	listenAddr      string
	advertiseAddr   string
	stop            bool
	err             error
	cancelDelay     func()
	attachers       map[string]*attacher
}

// attacher manages the in-memory attachment state of a container
// attachment to a global scope network managed by swarm manager. It
// helps in identifying the attachment ID via the taskID and the
// corresponding attachment configuration obtained from the manager.
type attacher struct {
	taskID           string
	config           *network.NetworkingConfig
	attachWaitCh     chan *network.NetworkingConfig
	attachCompleteCh chan struct{}
	detachWaitCh     chan struct{}
}

type node struct {
	*swarmagent.Node
	done           chan struct{}
	ready          bool
	conn           *grpc.ClientConn
	client         swarmapi.ControlClient
	reconnectDelay time.Duration
}

// New creates a new Cluster instance using provided config.
func New(config Config) (*Cluster, error) {
	root := filepath.Join(config.Root, swarmDirName)
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	if config.RuntimeRoot == "" {
		config.RuntimeRoot = root
	}
	if err := os.MkdirAll(config.RuntimeRoot, 0700); err != nil {
		return nil, err
	}
	c := &Cluster{
		root:        root,
		config:      config,
		configEvent: make(chan struct{}, 10),
		runtimeRoot: config.RuntimeRoot,
		attachers:   make(map[string]*attacher),
	}

	st, err := c.loadState()
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}

	n, err := c.startNewNode(false, st.LocalAddr, st.RemoteAddr, st.ListenAddr, st.AdvertiseAddr, "", "")
	if err != nil {
		return nil, err
	}

	select {
	case <-time.After(swarmConnectTimeout):
		logrus.Errorf("swarm component could not be started before timeout was reached")
	case <-n.Ready():
	case <-n.done:
		return nil, fmt.Errorf("swarm component could not be started: %v", c.err)
	}
	go c.reconnectOnFailure(n)
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
	dt, err := json.Marshal(state{
		LocalAddr:     c.localAddr,
		RemoteAddr:    c.remoteAddr,
		ListenAddr:    c.listenAddr,
		AdvertiseAddr: c.advertiseAddr,
	})
	if err != nil {
		return err
	}
	return ioutils.AtomicWriteFile(filepath.Join(c.root, stateFile), dt, 0600)
}

func (c *Cluster) reconnectOnFailure(n *node) {
	for {
		<-n.done
		c.Lock()
		if c.stop || c.node != nil {
			c.Unlock()
			return
		}
		n.reconnectDelay *= 2
		if n.reconnectDelay > maxReconnectDelay {
			n.reconnectDelay = maxReconnectDelay
		}
		logrus.Warnf("Restarting swarm in %.2f seconds", n.reconnectDelay.Seconds())
		delayCtx, cancel := context.WithTimeout(context.Background(), n.reconnectDelay)
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
		n, err = c.startNewNode(false, c.localAddr, c.getRemoteAddress(), c.listenAddr, c.advertiseAddr, c.getRemoteAddress(), "")
		if err != nil {
			c.err = err
			close(n.done)
		}
		c.Unlock()
	}
}

func (c *Cluster) startNewNode(forceNewCluster bool, localAddr, remoteAddr, listenAddr, advertiseAddr, joinAddr, joinToken string) (*node, error) {
	if err := c.config.Backend.IsSwarmCompatible(); err != nil {
		return nil, err
	}

	actualLocalAddr := localAddr
	if actualLocalAddr == "" {
		// If localAddr was not specified, resolve it automatically
		// based on the route to joinAddr. localAddr can only be left
		// empty on "join".
		listenHost, _, err := net.SplitHostPort(listenAddr)
		if err != nil {
			return nil, fmt.Errorf("could not parse listen address: %v", err)
		}

		listenAddrIP := net.ParseIP(listenHost)
		if listenAddrIP == nil || !listenAddrIP.IsUnspecified() {
			actualLocalAddr = listenHost
		} else {
			if remoteAddr == "" {
				// Should never happen except using swarms created by
				// old versions that didn't save remoteAddr.
				remoteAddr = "8.8.8.8:53"
			}
			conn, err := net.Dial("udp", remoteAddr)
			if err != nil {
				return nil, fmt.Errorf("could not find local IP address: %v", err)
			}
			localHostPort := conn.LocalAddr().String()
			actualLocalAddr, _, _ = net.SplitHostPort(localHostPort)
			conn.Close()
		}
	}

	c.node = nil
	c.cancelDelay = nil
	c.stop = false
	n, err := swarmagent.NewNode(&swarmagent.NodeConfig{
		Hostname:           c.config.Name,
		ForceNewCluster:    forceNewCluster,
		ListenControlAPI:   filepath.Join(c.runtimeRoot, controlSocket),
		ListenRemoteAPI:    listenAddr,
		AdvertiseRemoteAPI: advertiseAddr,
		JoinAddr:           joinAddr,
		StateDir:           c.root,
		JoinToken:          joinToken,
		Executor:           container.NewExecutor(c.config.Backend),
		HeartbeatTick:      1,
		ElectionTick:       3,
	})
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	if err := n.Start(ctx); err != nil {
		return nil, err
	}
	node := &node{
		Node:           n,
		done:           make(chan struct{}),
		reconnectDelay: initialReconnectDelay,
	}
	c.node = node
	c.localAddr = localAddr
	c.actualLocalAddr = actualLocalAddr // not saved
	c.remoteAddr = remoteAddr
	c.listenAddr = listenAddr
	c.advertiseAddr = advertiseAddr
	c.saveState()

	c.config.Backend.SetClusterProvider(c)
	go func() {
		err := n.Err(ctx)
		if err != nil {
			logrus.Errorf("cluster exited with error: %v", err)
		}
		c.Lock()
		c.node = nil
		c.err = err
		c.Unlock()
		close(node.done)
	}()

	go func() {
		select {
		case <-n.Ready():
			c.Lock()
			node.ready = true
			c.err = nil
			c.Unlock()
		case <-ctx.Done():
		}
		c.configEvent <- struct{}{}
	}()

	go func() {
		for conn := range n.ListenControlSocket(ctx) {
			c.Lock()
			if node.conn != conn {
				if conn == nil {
					node.client = nil
				} else {
					node.client = swarmapi.NewControlClient(conn)
				}
			}
			node.conn = conn
			c.Unlock()
			c.configEvent <- struct{}{}
		}
	}()

	return node, nil
}

// Init initializes new cluster from user provided request.
func (c *Cluster) Init(req types.InitRequest) (string, error) {
	c.Lock()
	if node := c.node; node != nil {
		if !req.ForceNewCluster {
			c.Unlock()
			return "", ErrSwarmExists
		}
		if err := c.stopNode(); err != nil {
			c.Unlock()
			return "", err
		}
	}

	if err := validateAndSanitizeInitRequest(&req); err != nil {
		c.Unlock()
		return "", err
	}

	listenHost, listenPort, err := resolveListenAddr(req.ListenAddr)
	if err != nil {
		c.Unlock()
		return "", err
	}

	advertiseHost, advertisePort, err := c.resolveAdvertiseAddr(req.AdvertiseAddr, listenPort)
	if err != nil {
		c.Unlock()
		return "", err
	}

	localAddr := listenHost

	// If the advertise address is not one of the system's
	// addresses, we also require a listen address.
	listenAddrIP := net.ParseIP(listenHost)
	if listenAddrIP != nil && listenAddrIP.IsUnspecified() {
		advertiseIP := net.ParseIP(advertiseHost)
		if advertiseIP == nil {
			// not an IP
			c.Unlock()
			return "", errMustSpecifyListenAddr
		}

		systemIPs := listSystemIPs()

		found := false
		for _, systemIP := range systemIPs {
			if systemIP.Equal(advertiseIP) {
				found = true
				break
			}
		}
		if !found {
			c.Unlock()
			return "", errMustSpecifyListenAddr
		}
		localAddr = advertiseIP.String()
	}

	// todo: check current state existing
	n, err := c.startNewNode(req.ForceNewCluster, localAddr, "", net.JoinHostPort(listenHost, listenPort), net.JoinHostPort(advertiseHost, advertisePort), "", "")
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
		go c.reconnectOnFailure(n)
		return n.NodeID(), nil
	case <-n.done:
		c.RLock()
		defer c.RUnlock()
		if !req.ForceNewCluster { // if failure on first attempt don't keep state
			if err := c.clearState(); err != nil {
				return "", err
			}
		}
		return "", c.err
	}
}

// Join makes current Cluster part of an existing swarm cluster.
func (c *Cluster) Join(req types.JoinRequest) error {
	c.Lock()
	if node := c.node; node != nil {
		c.Unlock()
		return ErrSwarmExists
	}
	if err := validateAndSanitizeJoinRequest(&req); err != nil {
		c.Unlock()
		return err
	}

	listenHost, listenPort, err := resolveListenAddr(req.ListenAddr)
	if err != nil {
		c.Unlock()
		return err
	}

	var advertiseAddr string
	if req.AdvertiseAddr != "" {
		advertiseHost, advertisePort, err := c.resolveAdvertiseAddr(req.AdvertiseAddr, listenPort)
		// For joining, we don't need to provide an advertise address,
		// since the remote side can detect it.
		if err == nil {
			advertiseAddr = net.JoinHostPort(advertiseHost, advertisePort)
		}
	}

	// todo: check current state existing
	n, err := c.startNewNode(false, "", req.RemoteAddrs[0], net.JoinHostPort(listenHost, listenPort), advertiseAddr, req.RemoteAddrs[0], req.JoinToken)
	if err != nil {
		c.Unlock()
		return err
	}
	c.Unlock()

	select {
	case <-time.After(swarmConnectTimeout):
		// attempt to connect will continue in background, also reconnecting
		go c.reconnectOnFailure(n)
		return ErrSwarmJoinTimeoutReached
	case <-n.Ready():
		go c.reconnectOnFailure(n)
		return nil
	case <-n.done:
		c.RLock()
		defer c.RUnlock()
		return c.err
	}
}

// stopNode is a helper that stops the active c.node and waits until it has
// shut down. Call while keeping the cluster lock.
func (c *Cluster) stopNode() error {
	if c.node == nil {
		return nil
	}
	c.stop = true
	if c.cancelDelay != nil {
		c.cancelDelay()
		c.cancelDelay = nil
	}
	node := c.node
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// TODO: can't hold lock on stop because it calls back to network
	c.Unlock()
	defer c.Lock()
	if err := node.Stop(ctx); err != nil && !strings.Contains(err.Error(), "context canceled") {
		return err
	}
	<-node.done
	return nil
}

func removingManagerCausesLossOfQuorum(reachable, unreachable int) bool {
	return reachable-2 <= unreachable
}

func isLastManager(reachable, unreachable int) bool {
	return reachable == 1 && unreachable == 0
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
		msg := "You are attempting to leave the swarm on a node that is participating as a manager. "
		if c.isActiveManager() {
			active, reachable, unreachable, err := c.managerStats()
			if err == nil {
				if active && removingManagerCausesLossOfQuorum(reachable, unreachable) {
					if isLastManager(reachable, unreachable) {
						msg += "Removing the last manager erases all current state of the swarm. Use `--force` to ignore this message. "
						c.Unlock()
						return fmt.Errorf(msg)
					}
					msg += fmt.Sprintf("Removing this node leaves %v managers out of %v. Without a Raft quorum your swarm will be inaccessible. ", reachable-1, reachable+unreachable)
				}
			}
		} else {
			msg += "Doing so may lose the consensus of your cluster. "
		}

		msg += "The only way to restore a swarm that has lost consensus is to reinitialize it with `--force-new-cluster`. Use `--force` to suppress this message."
		c.Unlock()
		return fmt.Errorf(msg)
	}
	if err := c.stopNode(); err != nil {
		logrus.Errorf("failed to shut down cluster node: %v", err)
		signal.DumpStacks("")
		c.Unlock()
		return err
	}
	c.Unlock()
	if nodeID := node.NodeID(); nodeID != "" {
		nodeContainers, err := c.listContainerForNode(nodeID)
		if err != nil {
			return err
		}
		for _, id := range nodeContainers {
			if err := c.config.Backend.ContainerRm(id, &apitypes.ContainerRmConfig{ForceRemove: true}); err != nil {
				logrus.Errorf("error removing %v: %v", id, err)
			}
		}
	}
	c.configEvent <- struct{}{}
	// todo: cleanup optional?
	if err := c.clearState(); err != nil {
		return err
	}
	return nil
}

func (c *Cluster) listContainerForNode(nodeID string) ([]string, error) {
	var ids []string
	filters := filters.NewArgs()
	filters.Add("label", fmt.Sprintf("com.docker.swarm.node.id=%s", nodeID))
	containers, err := c.config.Backend.Containers(&apitypes.ContainerListOptions{
		Filter: filters,
	})
	if err != nil {
		return []string{}, err
	}
	for _, c := range containers {
		ids = append(ids, c.ID)
	}
	return ids, nil
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

func (c *Cluster) getRequestContext() (context.Context, func()) { // TODO: not needed when requests don't block on qourum lost
	return context.WithTimeout(context.Background(), swarmRequestTimeout)
}

// Inspect retrieves the configuration properties of a managed swarm cluster.
func (c *Cluster) Inspect() (types.Swarm, error) {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return types.Swarm{}, c.errNoManager()
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	swarm, err := getSwarm(ctx, c.client)
	if err != nil {
		return types.Swarm{}, err
	}

	if err != nil {
		return types.Swarm{}, err
	}

	return convert.SwarmFromGRPC(*swarm), nil
}

// Update updates configuration of a managed swarm cluster.
func (c *Cluster) Update(version uint64, spec types.Spec, flags types.UpdateFlags) error {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return c.errNoManager()
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	swarm, err := getSwarm(ctx, c.client)
	if err != nil {
		return err
	}

	// In update, client should provide the complete spec of the swarm, including
	// Name and Labels. If a field is specified with 0 or nil, then the default value
	// will be used to swarmkit.
	clusterSpec, err := convert.SwarmSpecToGRPC(spec)
	if err != nil {
		return err
	}

	_, err = c.client.UpdateCluster(
		ctx,
		&swarmapi.UpdateClusterRequest{
			ClusterID: swarm.ID,
			Spec:      &clusterSpec,
			ClusterVersion: &swarmapi.Version{
				Index: version,
			},
			Rotation: swarmapi.JoinTokenRotation{
				RotateWorkerToken:  flags.RotateWorkerToken,
				RotateManagerToken: flags.RotateManagerToken,
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
	return c.node != nil && c.ready
}

// GetLocalAddress returns the local address.
func (c *Cluster) GetLocalAddress() string {
	c.RLock()
	defer c.RUnlock()
	return c.actualLocalAddr
}

// GetListenAddress returns the listen address.
func (c *Cluster) GetListenAddress() string {
	c.RLock()
	defer c.RUnlock()
	return c.listenAddr
}

// GetAdvertiseAddress returns the remotely reachable address of this node.
func (c *Cluster) GetAdvertiseAddress() string {
	c.RLock()
	defer c.RUnlock()
	if c.advertiseAddr != "" {
		advertiseHost, _, _ := net.SplitHostPort(c.advertiseAddr)
		return advertiseHost
	}
	return c.actualLocalAddr
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
	info := types.Info{
		NodeAddr: c.GetAdvertiseAddress(),
	}

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

	ctx, cancel := c.getRequestContext()
	defer cancel()

	if c.isActiveManager() {
		info.ControlAvailable = true
		swarm, err := c.Inspect()
		if err != nil {
			info.Error = err.Error()
		}

		// Strip JoinTokens
		info.Cluster = swarm.ClusterInfo

		if r, err := c.client.ListNodes(ctx, &swarmapi.ListNodesRequest{}); err == nil {
			info.Nodes = len(r.Nodes)
			for _, n := range r.Nodes {
				if n.ManagerStatus != nil {
					info.Managers = info.Managers + 1
				}
			}
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
	return c.node != nil && c.conn != nil
}

// errNoManager returns error describing why manager commands can't be used.
// Call with read lock.
func (c *Cluster) errNoManager() error {
	if c.node == nil {
		return fmt.Errorf("This node is not a swarm manager. Use \"docker swarm init\" or \"docker swarm join\" to connect this node to swarm and try again.")
	}
	if c.node.Manager() != nil {
		return fmt.Errorf("This node is not a swarm manager. Manager is being prepared or has trouble connecting to the cluster.")
	}
	return fmt.Errorf("This node is not a swarm manager. Worker nodes can't be used to view or modify cluster state. Please run this command on a manager node or promote the current node to a manager.")
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
	ctx, cancel := c.getRequestContext()
	defer cancel()

	r, err := c.client.ListServices(
		ctx,
		&swarmapi.ListServicesRequest{Filters: filters})
	if err != nil {
		return nil, err
	}

	services := []types.Service{}

	for _, service := range r.Services {
		services = append(services, convert.ServiceFromGRPC(*service))
	}

	return services, nil
}

// CreateService creates a new service in a managed swarm cluster.
func (c *Cluster) CreateService(s types.ServiceSpec, encodedAuth string) (string, error) {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return "", c.errNoManager()
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	err := c.populateNetworkID(ctx, c.client, &s)
	if err != nil {
		return "", err
	}

	serviceSpec, err := convert.ServiceSpecToGRPC(s)
	if err != nil {
		return "", err
	}

	if encodedAuth != "" {
		ctnr := serviceSpec.Task.GetContainer()
		if ctnr == nil {
			return "", fmt.Errorf("service does not use container tasks")
		}
		ctnr.PullOptions = &swarmapi.ContainerSpec_PullOptions{RegistryAuth: encodedAuth}
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

	ctx, cancel := c.getRequestContext()
	defer cancel()

	service, err := getService(ctx, c.client, input)
	if err != nil {
		return types.Service{}, err
	}
	return convert.ServiceFromGRPC(*service), nil
}

// UpdateService updates existing service to match new properties.
func (c *Cluster) UpdateService(serviceIDOrName string, version uint64, spec types.ServiceSpec, encodedAuth string) error {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return c.errNoManager()
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	err := c.populateNetworkID(ctx, c.client, &spec)
	if err != nil {
		return err
	}

	serviceSpec, err := convert.ServiceSpecToGRPC(spec)
	if err != nil {
		return err
	}

	currentService, err := getService(ctx, c.client, serviceIDOrName)
	if err != nil {
		return err
	}

	if encodedAuth != "" {
		ctnr := serviceSpec.Task.GetContainer()
		if ctnr == nil {
			return fmt.Errorf("service does not use container tasks")
		}
		ctnr.PullOptions = &swarmapi.ContainerSpec_PullOptions{RegistryAuth: encodedAuth}
	} else {
		// this is needed because if the encodedAuth isn't being updated then we
		// shouldn't lose it, and continue to use the one that was already present
		ctnr := currentService.Spec.Task.GetContainer()
		if ctnr == nil {
			return fmt.Errorf("service does not use container tasks")
		}
		serviceSpec.Task.GetContainer().PullOptions = ctnr.PullOptions
	}

	_, err = c.client.UpdateService(
		ctx,
		&swarmapi.UpdateServiceRequest{
			ServiceID: currentService.ID,
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

	ctx, cancel := c.getRequestContext()
	defer cancel()

	service, err := getService(ctx, c.client, input)
	if err != nil {
		return err
	}

	if _, err := c.client.RemoveService(ctx, &swarmapi.RemoveServiceRequest{ServiceID: service.ID}); err != nil {
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

	ctx, cancel := c.getRequestContext()
	defer cancel()

	r, err := c.client.ListNodes(
		ctx,
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

	ctx, cancel := c.getRequestContext()
	defer cancel()

	node, err := getNode(ctx, c.client, input)
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

	ctx, cancel := c.getRequestContext()
	defer cancel()

	_, err = c.client.UpdateNode(
		ctx,
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
func (c *Cluster) RemoveNode(input string, force bool) error {
	c.RLock()
	defer c.RUnlock()

	if !c.isActiveManager() {
		return c.errNoManager()
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	node, err := getNode(ctx, c.client, input)
	if err != nil {
		return err
	}

	if _, err := c.client.RemoveNode(ctx, &swarmapi.RemoveNodeRequest{NodeID: node.ID, Force: force}); err != nil {
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

	byName := func(filter filters.Args) error {
		if filter.Include("service") {
			serviceFilters := filter.Get("service")
			for _, serviceFilter := range serviceFilters {
				service, err := c.GetService(serviceFilter)
				if err != nil {
					return err
				}
				filter.Del("service", serviceFilter)
				filter.Add("service", service.ID)
			}
		}
		if filter.Include("node") {
			nodeFilters := filter.Get("node")
			for _, nodeFilter := range nodeFilters {
				node, err := c.GetNode(nodeFilter)
				if err != nil {
					return err
				}
				filter.Del("node", nodeFilter)
				filter.Add("node", node.ID)
			}
		}
		return nil
	}

	filters, err := newListTasksFilters(options.Filter, byName)
	if err != nil {
		return nil, err
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	r, err := c.client.ListTasks(
		ctx,
		&swarmapi.ListTasksRequest{Filters: filters})
	if err != nil {
		return nil, err
	}

	tasks := []types.Task{}

	for _, task := range r.Tasks {
		if task.Spec.GetContainer() != nil {
			tasks = append(tasks, convert.TaskFromGRPC(*task))
		}
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

	ctx, cancel := c.getRequestContext()
	defer cancel()

	task, err := getTask(ctx, c.client, input)
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

	ctx, cancel := c.getRequestContext()
	defer cancel()

	network, err := getNetwork(ctx, c.client, input)
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

	ctx, cancel := c.getRequestContext()
	defer cancel()

	r, err := c.client.ListNetworks(ctx, &swarmapi.ListNetworksRequest{})
	if err != nil {
		return nil, err
	}

	var networks []apitypes.NetworkResource

	for _, network := range r.Networks {
		networks = append(networks, convert.BasicNetworkFromGRPC(*network))
	}

	return networks, nil
}

func attacherKey(target, containerID string) string {
	return containerID + ":" + target
}

// UpdateAttachment signals the attachment config to the attachment
// waiter who is trying to start or attach the container to the
// network.
func (c *Cluster) UpdateAttachment(target, containerID string, config *network.NetworkingConfig) error {
	c.RLock()
	attacher, ok := c.attachers[attacherKey(target, containerID)]
	c.RUnlock()
	if !ok || attacher == nil {
		return fmt.Errorf("could not find attacher for container %s to network %s", containerID, target)
	}

	attacher.attachWaitCh <- config
	close(attacher.attachWaitCh)
	return nil
}

// WaitForDetachment waits for the container to stop or detach from
// the network.
func (c *Cluster) WaitForDetachment(ctx context.Context, networkName, networkID, taskID, containerID string) error {
	c.RLock()
	attacher, ok := c.attachers[attacherKey(networkName, containerID)]
	if !ok {
		attacher, ok = c.attachers[attacherKey(networkID, containerID)]
	}
	if c.node == nil || c.node.Agent() == nil {
		c.RUnlock()
		return fmt.Errorf("invalid cluster node while waiting for detachment")
	}

	agent := c.node.Agent()
	c.RUnlock()

	if ok && attacher != nil &&
		attacher.detachWaitCh != nil &&
		attacher.attachCompleteCh != nil {
		// Attachment may be in progress still so wait for
		// attachment to complete.
		select {
		case <-attacher.attachCompleteCh:
		case <-ctx.Done():
			return ctx.Err()
		}

		if attacher.taskID == taskID {
			select {
			case <-attacher.detachWaitCh:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return agent.ResourceAllocator().DetachNetwork(ctx, taskID)
}

// AttachNetwork generates an attachment request towards the manager.
func (c *Cluster) AttachNetwork(target string, containerID string, addresses []string) (*network.NetworkingConfig, error) {
	aKey := attacherKey(target, containerID)
	c.Lock()
	if c.node == nil || c.node.Agent() == nil {
		c.Unlock()
		return nil, fmt.Errorf("invalid cluster node while attaching to network")
	}
	if attacher, ok := c.attachers[aKey]; ok {
		c.Unlock()
		return attacher.config, nil
	}

	agent := c.node.Agent()
	attachWaitCh := make(chan *network.NetworkingConfig)
	detachWaitCh := make(chan struct{})
	attachCompleteCh := make(chan struct{})
	c.attachers[aKey] = &attacher{
		attachWaitCh:     attachWaitCh,
		attachCompleteCh: attachCompleteCh,
		detachWaitCh:     detachWaitCh,
	}
	c.Unlock()

	ctx, cancel := c.getRequestContext()
	defer cancel()

	taskID, err := agent.ResourceAllocator().AttachNetwork(ctx, containerID, target, addresses)
	if err != nil {
		c.Lock()
		delete(c.attachers, aKey)
		c.Unlock()
		return nil, fmt.Errorf("Could not attach to network %s: %v", target, err)
	}

	c.Lock()
	c.attachers[aKey].taskID = taskID
	close(attachCompleteCh)
	c.Unlock()

	logrus.Debugf("Successfully attached to network %s with tid %s", target, taskID)

	var config *network.NetworkingConfig
	select {
	case config = <-attachWaitCh:
	case <-ctx.Done():
		return nil, fmt.Errorf("attaching to network failed, make sure your network options are correct and check manager logs: %v", ctx.Err())
	}

	c.Lock()
	c.attachers[aKey].config = config
	c.Unlock()
	return config, nil
}

// DetachNetwork unblocks the waiters waiting on WaitForDetachment so
// that a request to detach can be generated towards the manager.
func (c *Cluster) DetachNetwork(target string, containerID string) error {
	aKey := attacherKey(target, containerID)

	c.Lock()
	attacher, ok := c.attachers[aKey]
	delete(c.attachers, aKey)
	c.Unlock()

	if !ok {
		return fmt.Errorf("could not find network attachment for container %s to network %s", containerID, target)
	}

	close(attacher.detachWaitCh)
	return nil
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

	ctx, cancel := c.getRequestContext()
	defer cancel()

	networkSpec := convert.BasicNetworkCreateToGRPC(s)
	r, err := c.client.CreateNetwork(ctx, &swarmapi.CreateNetworkRequest{Spec: &networkSpec})
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

	ctx, cancel := c.getRequestContext()
	defer cancel()

	network, err := getNetwork(ctx, c.client, input)
	if err != nil {
		return err
	}

	if _, err := c.client.RemoveNetwork(ctx, &swarmapi.RemoveNetworkRequest{NetworkID: network.ID}); err != nil {
		return err
	}
	return nil
}

func (c *Cluster) populateNetworkID(ctx context.Context, client swarmapi.ControlClient, s *types.ServiceSpec) error {
	// Always prefer NetworkAttachmentConfigs from TaskTemplate
	// but fallback to service spec for backward compatibility
	networks := s.TaskTemplate.Networks
	if len(networks) == 0 {
		networks = s.Networks
	}

	for i, n := range networks {
		apiNetwork, err := getNetwork(ctx, client, n.Target)
		if err != nil {
			if ln, _ := c.config.Backend.FindNetwork(n.Target); ln != nil && !ln.Info().Dynamic() {
				err = fmt.Errorf("network %s is not eligible for docker services", ln.Name())
				return errors.NewRequestForbiddenError(err)
			}
			return err
		}
		networks[i].Target = apiNetwork.ID
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
			return nil, fmt.Errorf("network %s is ambiguous (%d matches found)", input, l)
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
	defer c.Unlock()
	if c.isActiveManager() {
		active, reachable, unreachable, err := c.managerStats()
		if err == nil {
			singlenode := active && isLastManager(reachable, unreachable)
			if active && !singlenode && removingManagerCausesLossOfQuorum(reachable, unreachable) {
				logrus.Errorf("Leaving cluster with %v managers left out of %v. Raft quorum will be lost.", reachable-1, reachable+unreachable)
			}
		}
	}
	c.stopNode()
}

func (c *Cluster) managerStats() (current bool, reachable int, unreachable int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
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

func initClusterSpec(node *node, spec types.Spec) error {
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
			// In init, we take the initial default values from swarmkit, and merge
			// any non nil or 0 value from spec to GRPC spec. This will leave the
			// default value alone.
			// Note that this is different from Update(), as in Update() we expect
			// user to specify the complete spec of the cluster (as they already know
			// the existing one and knows which field to update)
			clusterSpec, err := convert.MergeSwarmSpecToGRPC(spec, cluster.Spec)
			if err != nil {
				return fmt.Errorf("error updating cluster settings: %v", err)
			}
			_, err = client.UpdateCluster(ctx, &swarmapi.UpdateClusterRequest{
				ClusterID:      cluster.ID,
				ClusterVersion: &cluster.Meta.Version,
				Spec:           &clusterSpec,
			})
			if err != nil {
				return fmt.Errorf("error updating cluster settings: %v", err)
			}
			return nil
		}
	}
	return ctx.Err()
}
