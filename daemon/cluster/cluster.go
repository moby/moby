package cluster

//
// ## Swarmkit integration
//
// Cluster - static configurable object for accessing everything swarm related.
// Contains methods for connecting and controlling the cluster. Exists always,
// even if swarm mode is not enabled.
//
// NodeRunner - Manager for starting the swarmkit node. Is present only and
// always if swarm mode is enabled. Implements backoff restart loop in case of
// errors.
//
// NodeState - Information about the current node status including access to
// gRPC clients if a manager is active.
//
// ### Locking
//
// `cluster.controlMutex` - taken for the whole lifecycle of the processes that
// can reconfigure cluster(init/join/leave etc). Protects that one
// reconfiguration action has fully completed before another can start.
//
// `cluster.mu` - taken when the actual changes in cluster configurations
// happen. Different from `controlMutex` because in some cases we need to
// access current cluster state even if the long-running reconfiguration is
// going on. For example network stack may ask for the current cluster state in
// the middle of the shutdown. Any time current cluster state is asked you
// should take the read lock of `cluster.mu`. If you are writing an API
// responder that returns synchronously, hold `cluster.mu.RLock()` for the
// duration of the whole handler function. That ensures that node will not be
// shut down until the handler has finished.
//
// NodeRunner implements its internal locks that should not be used outside of
// the struct. Instead, you should just call `nodeRunner.State()` method to get
// the current state of the cluster(still need `cluster.mu.RLock()` to access
// `cluster.nr` reference itself). Most of the changes in NodeRunner happen
// because of an external event(network problem, unexpected swarmkit error) and
// Docker shouldn't take any locks that delay these changes from happening.
//

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	distreference "github.com/docker/distribution/reference"
	apierrors "github.com/docker/docker/api/errors"
	apitypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	types "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/daemon/cluster/convert"
	executorpkg "github.com/docker/docker/daemon/cluster/executor"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/runconfig"
	swarmapi "github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/encryption"
	swarmnode "github.com/docker/swarmkit/node"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
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
	contextPrefix         = "com.docker.swarm"
)

// errNoSwarm is returned on leaving a cluster that was never initialized
var errNoSwarm = errors.New("This node is not part of a swarm")

// errSwarmExists is returned on initialize or join request for a cluster that has already been activated
var errSwarmExists = errors.New("This node is already part of a swarm. Use \"docker swarm leave\" to leave this swarm and join another one.")

// errSwarmJoinTimeoutReached is returned when cluster join could not complete before timeout was reached.
var errSwarmJoinTimeoutReached = errors.New("Timeout was reached before node was joined. The attempt to join the swarm will continue in the background. Use the \"docker info\" command to see the current swarm status of your node.")

// errSwarmLocked is returned if the swarm is encrypted and needs a key to unlock it.
var errSwarmLocked = errors.New("Swarm is encrypted and needs to be unlocked before it can be used. Please use \"docker swarm unlock\" to unlock it.")

// errSwarmCertificatesExpired is returned if docker was not started for the whole validity period and they had no chance to renew automatically.
var errSwarmCertificatesExpired = errors.New("Swarm certificates have expired. To replace them, leave the swarm and join again.")

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
	mu           sync.RWMutex
	controlMutex sync.RWMutex // protect init/join/leave user operations
	nr           *nodeRunner
	root         string
	runtimeRoot  string
	config       Config
	configEvent  chan struct{} // todo: make this array and goroutine safe
	attachers    map[string]*attacher
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

	nodeConfig, err := loadPersistentState(root)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}

	nr, err := c.newNodeRunner(*nodeConfig)
	if err != nil {
		return nil, err
	}
	c.nr = nr

	select {
	case <-time.After(swarmConnectTimeout):
		logrus.Error("swarm component could not be started before timeout was reached")
	case err := <-nr.Ready():
		if err != nil {
			if errors.Cause(err) == errSwarmLocked {
				return c, nil
			}
			if err, ok := errors.Cause(c.nr.err).(x509.CertificateInvalidError); ok && err.Reason == x509.Expired {
				return c, nil
			}
			return nil, errors.Wrap(err, "swarm component could not be started")
		}
	}
	return c, nil
}

func (c *Cluster) newNodeRunner(conf nodeStartConfig) (*nodeRunner, error) {
	if err := c.config.Backend.IsSwarmCompatible(); err != nil {
		return nil, err
	}

	actualLocalAddr := conf.LocalAddr
	if actualLocalAddr == "" {
		// If localAddr was not specified, resolve it automatically
		// based on the route to joinAddr. localAddr can only be left
		// empty on "join".
		listenHost, _, err := net.SplitHostPort(conf.ListenAddr)
		if err != nil {
			return nil, fmt.Errorf("could not parse listen address: %v", err)
		}

		listenAddrIP := net.ParseIP(listenHost)
		if listenAddrIP == nil || !listenAddrIP.IsUnspecified() {
			actualLocalAddr = listenHost
		} else {
			if conf.RemoteAddr == "" {
				// Should never happen except using swarms created by
				// old versions that didn't save remoteAddr.
				conf.RemoteAddr = "8.8.8.8:53"
			}
			conn, err := net.Dial("udp", conf.RemoteAddr)
			if err != nil {
				return nil, fmt.Errorf("could not find local IP address: %v", err)
			}
			localHostPort := conn.LocalAddr().String()
			actualLocalAddr, _, _ = net.SplitHostPort(localHostPort)
			conn.Close()
		}
	}

	nr := &nodeRunner{cluster: c}
	nr.actualLocalAddr = actualLocalAddr

	if err := nr.Start(conf); err != nil {
		return nil, err
	}

	c.config.Backend.DaemonJoinsCluster(c)

	return nr, nil
}

// Init initializes new cluster from user provided request.
func (c *Cluster) Init(req types.InitRequest) (string, error) {
	c.controlMutex.Lock()
	defer c.controlMutex.Unlock()
	c.mu.Lock()
	if c.nr != nil {
		if req.ForceNewCluster {
			if err := c.nr.Stop(); err != nil {
				c.mu.Unlock()
				return "", err
			}
		} else {
			c.mu.Unlock()
			return "", errSwarmExists
		}
	}
	c.mu.Unlock()

	if err := validateAndSanitizeInitRequest(&req); err != nil {
		return "", apierrors.NewBadRequestError(err)
	}

	listenHost, listenPort, err := resolveListenAddr(req.ListenAddr)
	if err != nil {
		return "", err
	}

	advertiseHost, advertisePort, err := c.resolveAdvertiseAddr(req.AdvertiseAddr, listenPort)
	if err != nil {
		return "", err
	}

	localAddr := listenHost

	// If the local address is undetermined, the advertise address
	// will be used as local address, if it belongs to this system.
	// If the advertise address is not local, then we try to find
	// a system address to use as local address. If this fails,
	// we give up and ask the user to pass the listen address.
	if net.ParseIP(localAddr).IsUnspecified() {
		advertiseIP := net.ParseIP(advertiseHost)

		found := false
		for _, systemIP := range listSystemIPs() {
			if systemIP.Equal(advertiseIP) {
				localAddr = advertiseIP.String()
				found = true
				break
			}
		}

		if !found {
			ip, err := c.resolveSystemAddr()
			if err != nil {
				logrus.Warnf("Could not find a local address: %v", err)
				return "", errMustSpecifyListenAddr
			}
			localAddr = ip.String()
		}
	}

	if !req.ForceNewCluster {
		clearPersistentState(c.root)
	}

	nr, err := c.newNodeRunner(nodeStartConfig{
		forceNewCluster: req.ForceNewCluster,
		autolock:        req.AutoLockManagers,
		LocalAddr:       localAddr,
		ListenAddr:      net.JoinHostPort(listenHost, listenPort),
		AdvertiseAddr:   net.JoinHostPort(advertiseHost, advertisePort),
		availability:    req.Availability,
	})
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	c.nr = nr
	c.mu.Unlock()

	if err := <-nr.Ready(); err != nil {
		if !req.ForceNewCluster { // if failure on first attempt don't keep state
			if err := clearPersistentState(c.root); err != nil {
				return "", err
			}
		}
		if err != nil {
			c.mu.Lock()
			c.nr = nil
			c.mu.Unlock()
		}
		return "", err
	}
	state := nr.State()
	if state.swarmNode == nil { // should never happen but protect from panic
		return "", errors.New("invalid cluster state for spec initialization")
	}
	if err := initClusterSpec(state.swarmNode, req.Spec); err != nil {
		return "", err
	}
	return state.NodeID(), nil
}

// Join makes current Cluster part of an existing swarm cluster.
func (c *Cluster) Join(req types.JoinRequest) error {
	c.controlMutex.Lock()
	defer c.controlMutex.Unlock()
	c.mu.Lock()
	if c.nr != nil {
		c.mu.Unlock()
		return errSwarmExists
	}
	c.mu.Unlock()

	if err := validateAndSanitizeJoinRequest(&req); err != nil {
		return apierrors.NewBadRequestError(err)
	}

	listenHost, listenPort, err := resolveListenAddr(req.ListenAddr)
	if err != nil {
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

	clearPersistentState(c.root)

	nr, err := c.newNodeRunner(nodeStartConfig{
		RemoteAddr:    req.RemoteAddrs[0],
		ListenAddr:    net.JoinHostPort(listenHost, listenPort),
		AdvertiseAddr: advertiseAddr,
		joinAddr:      req.RemoteAddrs[0],
		joinToken:     req.JoinToken,
		availability:  req.Availability,
	})
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.nr = nr
	c.mu.Unlock()

	select {
	case <-time.After(swarmConnectTimeout):
		return errSwarmJoinTimeoutReached
	case err := <-nr.Ready():
		if err != nil {
			c.mu.Lock()
			c.nr = nil
			c.mu.Unlock()
		}
		return err
	}
}

// GetUnlockKey returns the unlock key for the swarm.
func (c *Cluster) GetUnlockKey() (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return "", c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	client := swarmapi.NewCAClient(state.grpcConn)

	r, err := client.GetUnlockKey(ctx, &swarmapi.GetUnlockKeyRequest{})
	if err != nil {
		return "", err
	}

	if len(r.UnlockKey) == 0 {
		// no key
		return "", nil
	}

	return encryption.HumanReadableKey(r.UnlockKey), nil
}

// UnlockSwarm provides a key to decrypt data that is encrypted at rest.
func (c *Cluster) UnlockSwarm(req types.UnlockRequest) error {
	c.controlMutex.Lock()
	defer c.controlMutex.Unlock()

	c.mu.RLock()
	state := c.currentNodeState()

	if !state.IsActiveManager() {
		// when manager is not active,
		// unless it is locked, otherwise return error.
		if err := c.errNoManager(state); err != errSwarmLocked {
			c.mu.RUnlock()
			return err
		}
	} else {
		// when manager is active, return an error of "not locked"
		c.mu.RUnlock()
		return errors.New("swarm is not locked")
	}

	// only when swarm is locked, code running reaches here
	nr := c.nr
	c.mu.RUnlock()

	key, err := encryption.ParseHumanReadableKey(req.UnlockKey)
	if err != nil {
		return err
	}

	config := nr.config
	config.lockKey = key
	if err := nr.Stop(); err != nil {
		return err
	}
	nr, err = c.newNodeRunner(config)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.nr = nr
	c.mu.Unlock()

	if err := <-nr.Ready(); err != nil {
		if errors.Cause(err) == errSwarmLocked {
			return errors.New("swarm could not be unlocked: invalid key provided")
		}
		return fmt.Errorf("swarm component could not be started: %v", err)
	}
	return nil
}

// Leave shuts down Cluster and removes current state.
func (c *Cluster) Leave(force bool) error {
	c.controlMutex.Lock()
	defer c.controlMutex.Unlock()

	c.mu.Lock()
	nr := c.nr
	if nr == nil {
		c.mu.Unlock()
		return errNoSwarm
	}

	state := c.currentNodeState()

	if errors.Cause(state.err) == errSwarmLocked && !force {
		// leave a locked swarm without --force is not allowed
		c.mu.Unlock()
		return errors.New("Swarm is encrypted and locked. Please unlock it first or use `--force` to ignore this message.")
	}

	if state.IsManager() && !force {
		msg := "You are attempting to leave the swarm on a node that is participating as a manager. "
		if state.IsActiveManager() {
			active, reachable, unreachable, err := managerStats(state.controlClient, state.NodeID())
			if err == nil {
				if active && removingManagerCausesLossOfQuorum(reachable, unreachable) {
					if isLastManager(reachable, unreachable) {
						msg += "Removing the last manager erases all current state of the swarm. Use `--force` to ignore this message. "
						c.mu.Unlock()
						return errors.New(msg)
					}
					msg += fmt.Sprintf("Removing this node leaves %v managers out of %v. Without a Raft quorum your swarm will be inaccessible. ", reachable-1, reachable+unreachable)
				}
			}
		} else {
			msg += "Doing so may lose the consensus of your cluster. "
		}

		msg += "The only way to restore a swarm that has lost consensus is to reinitialize it with `--force-new-cluster`. Use `--force` to suppress this message."
		c.mu.Unlock()
		return errors.New(msg)
	}
	// release readers in here
	if err := nr.Stop(); err != nil {
		logrus.Errorf("failed to shut down cluster node: %v", err)
		signal.DumpStacks("")
		c.mu.Unlock()
		return err
	}
	c.nr = nil
	c.mu.Unlock()
	if nodeID := state.NodeID(); nodeID != "" {
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
	if err := clearPersistentState(c.root); err != nil {
		return err
	}
	c.config.Backend.DaemonLeavesCluster()
	return nil
}

func (c *Cluster) listContainerForNode(nodeID string) ([]string, error) {
	var ids []string
	filters := filters.NewArgs()
	filters.Add("label", fmt.Sprintf("com.docker.swarm.node.id=%s", nodeID))
	containers, err := c.config.Backend.Containers(&apitypes.ContainerListOptions{
		Filters: filters,
	})
	if err != nil {
		return []string{}, err
	}
	for _, c := range containers {
		ids = append(ids, c.ID)
	}
	return ids, nil
}

func (c *Cluster) getRequestContext() (context.Context, func()) { // TODO: not needed when requests don't block on qourum lost
	return context.WithTimeout(context.Background(), swarmRequestTimeout)
}

// Inspect retrieves the configuration properties of a managed swarm cluster.
func (c *Cluster) Inspect() (types.Swarm, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return types.Swarm{}, c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	swarm, err := getSwarm(ctx, state.controlClient)
	if err != nil {
		return types.Swarm{}, err
	}

	return convert.SwarmFromGRPC(*swarm), nil
}

// Update updates configuration of a managed swarm cluster.
func (c *Cluster) Update(version uint64, spec types.Spec, flags types.UpdateFlags) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	swarm, err := getSwarm(ctx, state.controlClient)
	if err != nil {
		return err
	}

	// In update, client should provide the complete spec of the swarm, including
	// Name and Labels. If a field is specified with 0 or nil, then the default value
	// will be used to swarmkit.
	clusterSpec, err := convert.SwarmSpecToGRPC(spec)
	if err != nil {
		return apierrors.NewBadRequestError(err)
	}

	_, err = state.controlClient.UpdateCluster(
		ctx,
		&swarmapi.UpdateClusterRequest{
			ClusterID: swarm.ID,
			Spec:      &clusterSpec,
			ClusterVersion: &swarmapi.Version{
				Index: version,
			},
			Rotation: swarmapi.KeyRotation{
				WorkerJoinToken:  flags.RotateWorkerToken,
				ManagerJoinToken: flags.RotateManagerToken,
				ManagerUnlockKey: flags.RotateManagerUnlockKey,
			},
		},
	)
	return err
}

// IsManager returns true if Cluster is participating as a manager.
func (c *Cluster) IsManager() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentNodeState().IsActiveManager()
}

// IsAgent returns true if Cluster is participating as a worker/agent.
func (c *Cluster) IsAgent() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentNodeState().status == types.LocalNodeStateActive
}

// GetLocalAddress returns the local address.
func (c *Cluster) GetLocalAddress() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentNodeState().actualLocalAddr
}

// GetListenAddress returns the listen address.
func (c *Cluster) GetListenAddress() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.nr != nil {
		return c.nr.config.ListenAddr
	}
	return ""
}

// GetAdvertiseAddress returns the remotely reachable address of this node.
func (c *Cluster) GetAdvertiseAddress() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.nr != nil && c.nr.config.AdvertiseAddr != "" {
		advertiseHost, _, _ := net.SplitHostPort(c.nr.config.AdvertiseAddr)
		return advertiseHost
	}
	return c.currentNodeState().actualLocalAddr
}

// GetRemoteAddress returns a known advertise address of a remote manager if
// available.
// todo: change to array/connect with info
func (c *Cluster) GetRemoteAddress() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.getRemoteAddress()
}

func (c *Cluster) getRemoteAddress() string {
	state := c.currentNodeState()
	if state.swarmNode == nil {
		return ""
	}
	nodeID := state.swarmNode.NodeID()
	for _, r := range state.swarmNode.Remotes() {
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
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	info.LocalNodeState = state.status
	if state.err != nil {
		info.Error = state.err.Error()
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	if state.IsActiveManager() {
		info.ControlAvailable = true
		swarm, err := c.Inspect()
		if err != nil {
			info.Error = err.Error()
		}

		// Strip JoinTokens
		info.Cluster = swarm.ClusterInfo

		if r, err := state.controlClient.ListNodes(ctx, &swarmapi.ListNodesRequest{}); err != nil {
			info.Error = err.Error()
		} else {
			info.Nodes = len(r.Nodes)
			for _, n := range r.Nodes {
				if n.ManagerStatus != nil {
					info.Managers = info.Managers + 1
				}
			}
		}
	}

	if state.swarmNode != nil {
		for _, r := range state.swarmNode.Remotes() {
			info.RemoteManagers = append(info.RemoteManagers, types.Peer{NodeID: r.NodeID, Addr: r.Addr})
		}
		info.NodeID = state.swarmNode.NodeID()
	}

	return info
}

// currentNodeState should not be called without a read lock
func (c *Cluster) currentNodeState() nodeState {
	return c.nr.State()
}

// errNoManager returns error describing why manager commands can't be used.
// Call with read lock.
func (c *Cluster) errNoManager(st nodeState) error {
	if st.swarmNode == nil {
		if errors.Cause(st.err) == errSwarmLocked {
			return errSwarmLocked
		}
		if st.err == errSwarmCertificatesExpired {
			return errSwarmCertificatesExpired
		}
		return errors.New("This node is not a swarm manager. Use \"docker swarm init\" or \"docker swarm join\" to connect this node to swarm and try again.")
	}
	if st.swarmNode.Manager() != nil {
		return errors.New("This node is not a swarm manager. Manager is being prepared or has trouble connecting to the cluster.")
	}
	return errors.New("This node is not a swarm manager. Worker nodes can't be used to view or modify cluster state. Please run this command on a manager node or promote the current node to a manager.")
}

// GetServices returns all services of a managed swarm cluster.
func (c *Cluster) GetServices(options apitypes.ServiceListOptions) ([]types.Service, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return nil, c.errNoManager(state)
	}

	filters, err := newListServicesFilters(options.Filters)
	if err != nil {
		return nil, err
	}
	ctx, cancel := c.getRequestContext()
	defer cancel()

	r, err := state.controlClient.ListServices(
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

// imageWithDigestString takes an image such as name or name:tag
// and returns the image pinned to a digest, such as name@sha256:34234...
// Due to the difference between the docker/docker/reference, and the
// docker/distribution/reference packages, we're parsing the image twice.
// As the two packages converge, this function should be simplified.
// TODO(nishanttotla): After the packages converge, the function must
// convert distreference.Named -> distreference.Canonical, and the logic simplified.
func (c *Cluster) imageWithDigestString(ctx context.Context, image string, authConfig *apitypes.AuthConfig) (string, error) {
	if _, err := digest.Parse(image); err == nil {
		return "", errors.New("image reference is an image ID")
	}
	ref, err := distreference.ParseNamed(image)
	if err != nil {
		return "", err
	}
	// only query registry if not a canonical reference (i.e. with digest)
	if _, ok := ref.(distreference.Canonical); !ok {
		// create a docker/docker/reference Named object because GetRepository needs it
		dockerRef, err := reference.ParseNamed(image)
		if err != nil {
			return "", err
		}
		dockerRef = reference.WithDefaultTag(dockerRef)
		namedTaggedRef, ok := dockerRef.(reference.NamedTagged)
		if !ok {
			return "", errors.New("unable to cast image to NamedTagged reference object")
		}

		repo, _, err := c.config.Backend.GetRepository(ctx, namedTaggedRef, authConfig)
		if err != nil {
			return "", err
		}
		dscrptr, err := repo.Tags(ctx).Get(ctx, namedTaggedRef.Tag())
		if err != nil {
			return "", err
		}

		namedDigestedRef, err := distreference.WithDigest(distreference.EnsureTagged(ref), dscrptr.Digest)
		if err != nil {
			return "", err
		}
		return namedDigestedRef.String(), nil
	}
	// reference already contains a digest, so just return it
	return ref.String(), nil
}

// CreateService creates a new service in a managed swarm cluster.
func (c *Cluster) CreateService(s types.ServiceSpec, encodedAuth string) (*apitypes.ServiceCreateResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return nil, c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	err := c.populateNetworkID(ctx, state.controlClient, &s)
	if err != nil {
		return nil, err
	}

	serviceSpec, err := convert.ServiceSpecToGRPC(s)
	if err != nil {
		return nil, apierrors.NewBadRequestError(err)
	}

	ctnr := serviceSpec.Task.GetContainer()
	if ctnr == nil {
		return nil, errors.New("service does not use container tasks")
	}

	if encodedAuth != "" {
		ctnr.PullOptions = &swarmapi.ContainerSpec_PullOptions{RegistryAuth: encodedAuth}
	}

	// retrieve auth config from encoded auth
	authConfig := &apitypes.AuthConfig{}
	if encodedAuth != "" {
		if err := json.NewDecoder(base64.NewDecoder(base64.URLEncoding, strings.NewReader(encodedAuth))).Decode(authConfig); err != nil {
			logrus.Warnf("invalid authconfig: %v", err)
		}
	}

	resp := &apitypes.ServiceCreateResponse{}

	// pin image by digest
	if os.Getenv("DOCKER_SERVICE_PREFER_OFFLINE_IMAGE") != "1" {
		digestImage, err := c.imageWithDigestString(ctx, ctnr.Image, authConfig)
		if err != nil {
			logrus.Warnf("unable to pin image %s to digest: %s", ctnr.Image, err.Error())
			resp.Warnings = append(resp.Warnings, fmt.Sprintf("unable to pin image %s to digest: %s", ctnr.Image, err.Error()))
		} else if ctnr.Image != digestImage {
			logrus.Debugf("pinning image %s by digest: %s", ctnr.Image, digestImage)
			ctnr.Image = digestImage
		} else {
			logrus.Debugf("creating service using supplied digest reference %s", ctnr.Image)
		}
	}

	r, err := state.controlClient.CreateService(ctx, &swarmapi.CreateServiceRequest{Spec: &serviceSpec})
	if err != nil {
		return nil, err
	}

	resp.ID = r.Service.ID
	return resp, nil
}

// GetService returns a service based on an ID or name.
func (c *Cluster) GetService(input string) (types.Service, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return types.Service{}, c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	service, err := getService(ctx, state.controlClient, input)
	if err != nil {
		return types.Service{}, err
	}
	return convert.ServiceFromGRPC(*service), nil
}

// UpdateService updates existing service to match new properties.
func (c *Cluster) UpdateService(serviceIDOrName string, version uint64, spec types.ServiceSpec, encodedAuth string, registryAuthFrom string) (*apitypes.ServiceUpdateResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return nil, c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	err := c.populateNetworkID(ctx, state.controlClient, &spec)
	if err != nil {
		return nil, err
	}

	serviceSpec, err := convert.ServiceSpecToGRPC(spec)
	if err != nil {
		return nil, apierrors.NewBadRequestError(err)
	}

	currentService, err := getService(ctx, state.controlClient, serviceIDOrName)
	if err != nil {
		return nil, err
	}

	newCtnr := serviceSpec.Task.GetContainer()
	if newCtnr == nil {
		return nil, errors.New("service does not use container tasks")
	}

	if encodedAuth != "" {
		newCtnr.PullOptions = &swarmapi.ContainerSpec_PullOptions{RegistryAuth: encodedAuth}
	} else {
		// this is needed because if the encodedAuth isn't being updated then we
		// shouldn't lose it, and continue to use the one that was already present
		var ctnr *swarmapi.ContainerSpec
		switch registryAuthFrom {
		case apitypes.RegistryAuthFromSpec, "":
			ctnr = currentService.Spec.Task.GetContainer()
		case apitypes.RegistryAuthFromPreviousSpec:
			if currentService.PreviousSpec == nil {
				return nil, errors.New("service does not have a previous spec")
			}
			ctnr = currentService.PreviousSpec.Task.GetContainer()
		default:
			return nil, errors.New("unsupported registryAuthFrom value")
		}
		if ctnr == nil {
			return nil, errors.New("service does not use container tasks")
		}
		newCtnr.PullOptions = ctnr.PullOptions
		// update encodedAuth so it can be used to pin image by digest
		if ctnr.PullOptions != nil {
			encodedAuth = ctnr.PullOptions.RegistryAuth
		}
	}

	// retrieve auth config from encoded auth
	authConfig := &apitypes.AuthConfig{}
	if encodedAuth != "" {
		if err := json.NewDecoder(base64.NewDecoder(base64.URLEncoding, strings.NewReader(encodedAuth))).Decode(authConfig); err != nil {
			logrus.Warnf("invalid authconfig: %v", err)
		}
	}

	resp := &apitypes.ServiceUpdateResponse{}

	// pin image by digest
	if os.Getenv("DOCKER_SERVICE_PREFER_OFFLINE_IMAGE") != "1" {
		digestImage, err := c.imageWithDigestString(ctx, newCtnr.Image, authConfig)
		if err != nil {
			logrus.Warnf("unable to pin image %s to digest: %s", newCtnr.Image, err.Error())
			resp.Warnings = append(resp.Warnings, fmt.Sprintf("unable to pin image %s to digest: %s", newCtnr.Image, err.Error()))
		} else if newCtnr.Image != digestImage {
			logrus.Debugf("pinning image %s by digest: %s", newCtnr.Image, digestImage)
			newCtnr.Image = digestImage
		} else {
			logrus.Debugf("updating service using supplied digest reference %s", newCtnr.Image)
		}
	}

	_, err = state.controlClient.UpdateService(
		ctx,
		&swarmapi.UpdateServiceRequest{
			ServiceID: currentService.ID,
			Spec:      &serviceSpec,
			ServiceVersion: &swarmapi.Version{
				Index: version,
			},
		},
	)

	return resp, err
}

// RemoveService removes a service from a managed swarm cluster.
func (c *Cluster) RemoveService(input string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	service, err := getService(ctx, state.controlClient, input)
	if err != nil {
		return err
	}

	_, err = state.controlClient.RemoveService(ctx, &swarmapi.RemoveServiceRequest{ServiceID: service.ID})
	return err
}

// ServiceLogs collects service logs and writes them back to `config.OutStream`
func (c *Cluster) ServiceLogs(ctx context.Context, input string, config *backend.ContainerLogsConfig, started chan struct{}) error {
	c.mu.RLock()
	state := c.currentNodeState()
	if !state.IsActiveManager() {
		c.mu.RUnlock()
		return c.errNoManager(state)
	}

	service, err := getService(ctx, state.controlClient, input)
	if err != nil {
		c.mu.RUnlock()
		return err
	}

	stream, err := state.logsClient.SubscribeLogs(ctx, &swarmapi.SubscribeLogsRequest{
		Selector: &swarmapi.LogSelector{
			ServiceIDs: []string{service.ID},
		},
		Options: &swarmapi.LogSubscriptionOptions{
			Follow: config.Follow,
		},
	})
	if err != nil {
		c.mu.RUnlock()
		return err
	}

	wf := ioutils.NewWriteFlusher(config.OutStream)
	defer wf.Close()
	close(started)
	wf.Flush()

	outStream := stdcopy.NewStdWriter(wf, stdcopy.Stdout)
	errStream := stdcopy.NewStdWriter(wf, stdcopy.Stderr)

	// Release the lock before starting the stream.
	c.mu.RUnlock()
	for {
		// Check the context before doing anything.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		subscribeMsg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		for _, msg := range subscribeMsg.Messages {
			data := []byte{}

			if config.Timestamps {
				ts, err := gogotypes.TimestampFromProto(msg.Timestamp)
				if err != nil {
					return err
				}
				data = append(data, []byte(ts.Format(logger.TimeFormat)+" ")...)
			}

			data = append(data, []byte(fmt.Sprintf("%s.node.id=%s,%s.service.id=%s,%s.task.id=%s ",
				contextPrefix, msg.Context.NodeID,
				contextPrefix, msg.Context.ServiceID,
				contextPrefix, msg.Context.TaskID,
			))...)

			data = append(data, msg.Data...)

			switch msg.Stream {
			case swarmapi.LogStreamStdout:
				outStream.Write(data)
			case swarmapi.LogStreamStderr:
				errStream.Write(data)
			}
		}
	}
}

// GetNodes returns a list of all nodes known to a cluster.
func (c *Cluster) GetNodes(options apitypes.NodeListOptions) ([]types.Node, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return nil, c.errNoManager(state)
	}

	filters, err := newListNodesFilters(options.Filters)
	if err != nil {
		return nil, err
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	r, err := state.controlClient.ListNodes(
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

// GetNode returns a node based on an ID.
func (c *Cluster) GetNode(input string) (types.Node, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return types.Node{}, c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	node, err := getNode(ctx, state.controlClient, input)
	if err != nil {
		return types.Node{}, err
	}
	return convert.NodeFromGRPC(*node), nil
}

// UpdateNode updates existing nodes properties.
func (c *Cluster) UpdateNode(input string, version uint64, spec types.NodeSpec) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return c.errNoManager(state)
	}

	nodeSpec, err := convert.NodeSpecToGRPC(spec)
	if err != nil {
		return apierrors.NewBadRequestError(err)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	currentNode, err := getNode(ctx, state.controlClient, input)
	if err != nil {
		return err
	}

	_, err = state.controlClient.UpdateNode(
		ctx,
		&swarmapi.UpdateNodeRequest{
			NodeID: currentNode.ID,
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
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	node, err := getNode(ctx, state.controlClient, input)
	if err != nil {
		return err
	}

	_, err = state.controlClient.RemoveNode(ctx, &swarmapi.RemoveNodeRequest{NodeID: node.ID, Force: force})
	return err
}

// GetTasks returns a list of tasks matching the filter options.
func (c *Cluster) GetTasks(options apitypes.TaskListOptions) ([]types.Task, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return nil, c.errNoManager(state)
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

	filters, err := newListTasksFilters(options.Filters, byName)
	if err != nil {
		return nil, err
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	r, err := state.controlClient.ListTasks(
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
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return types.Task{}, c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	task, err := getTask(ctx, state.controlClient, input)
	if err != nil {
		return types.Task{}, err
	}
	return convert.TaskFromGRPC(*task), nil
}

// GetNetwork returns a cluster network by an ID.
func (c *Cluster) GetNetwork(input string) (apitypes.NetworkResource, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return apitypes.NetworkResource{}, c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	network, err := getNetwork(ctx, state.controlClient, input)
	if err != nil {
		return apitypes.NetworkResource{}, err
	}
	return convert.BasicNetworkFromGRPC(*network), nil
}

func (c *Cluster) getNetworks(filters *swarmapi.ListNetworksRequest_Filters) ([]apitypes.NetworkResource, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return nil, c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	r, err := state.controlClient.ListNetworks(ctx, &swarmapi.ListNetworksRequest{Filters: filters})
	if err != nil {
		return nil, err
	}

	var networks []apitypes.NetworkResource

	for _, network := range r.Networks {
		networks = append(networks, convert.BasicNetworkFromGRPC(*network))
	}

	return networks, nil
}

// GetNetworks returns all current cluster managed networks.
func (c *Cluster) GetNetworks() ([]apitypes.NetworkResource, error) {
	return c.getNetworks(nil)
}

// GetNetworksByName returns cluster managed networks by name.
// It is ok to have multiple networks here. #18864
func (c *Cluster) GetNetworksByName(name string) ([]apitypes.NetworkResource, error) {
	// Note that swarmapi.GetNetworkRequest.Name is not functional.
	// So we cannot just use that with c.GetNetwork.
	return c.getNetworks(&swarmapi.ListNetworksRequest_Filters{
		Names: []string{name},
	})
}

func attacherKey(target, containerID string) string {
	return containerID + ":" + target
}

// UpdateAttachment signals the attachment config to the attachment
// waiter who is trying to start or attach the container to the
// network.
func (c *Cluster) UpdateAttachment(target, containerID string, config *network.NetworkingConfig) error {
	c.mu.RLock()
	attacher, ok := c.attachers[attacherKey(target, containerID)]
	c.mu.RUnlock()
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
	c.mu.RLock()
	attacher, ok := c.attachers[attacherKey(networkName, containerID)]
	if !ok {
		attacher, ok = c.attachers[attacherKey(networkID, containerID)]
	}
	state := c.currentNodeState()
	if state.swarmNode == nil || state.swarmNode.Agent() == nil {
		c.mu.RUnlock()
		return errors.New("invalid cluster node while waiting for detachment")
	}

	c.mu.RUnlock()
	agent := state.swarmNode.Agent()
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
	c.mu.Lock()
	state := c.currentNodeState()
	if state.swarmNode == nil || state.swarmNode.Agent() == nil {
		c.mu.Unlock()
		return nil, errors.New("invalid cluster node while attaching to network")
	}
	if attacher, ok := c.attachers[aKey]; ok {
		c.mu.Unlock()
		return attacher.config, nil
	}

	agent := state.swarmNode.Agent()
	attachWaitCh := make(chan *network.NetworkingConfig)
	detachWaitCh := make(chan struct{})
	attachCompleteCh := make(chan struct{})
	c.attachers[aKey] = &attacher{
		attachWaitCh:     attachWaitCh,
		attachCompleteCh: attachCompleteCh,
		detachWaitCh:     detachWaitCh,
	}
	c.mu.Unlock()

	ctx, cancel := c.getRequestContext()
	defer cancel()

	taskID, err := agent.ResourceAllocator().AttachNetwork(ctx, containerID, target, addresses)
	if err != nil {
		c.mu.Lock()
		delete(c.attachers, aKey)
		c.mu.Unlock()
		return nil, fmt.Errorf("Could not attach to network %s: %v", target, err)
	}

	c.mu.Lock()
	c.attachers[aKey].taskID = taskID
	close(attachCompleteCh)
	c.mu.Unlock()

	logrus.Debugf("Successfully attached to network %s with tid %s", target, taskID)

	var config *network.NetworkingConfig
	select {
	case config = <-attachWaitCh:
	case <-ctx.Done():
		return nil, fmt.Errorf("attaching to network failed, make sure your network options are correct and check manager logs: %v", ctx.Err())
	}

	c.mu.Lock()
	c.attachers[aKey].config = config
	c.mu.Unlock()
	return config, nil
}

// DetachNetwork unblocks the waiters waiting on WaitForDetachment so
// that a request to detach can be generated towards the manager.
func (c *Cluster) DetachNetwork(target string, containerID string) error {
	aKey := attacherKey(target, containerID)

	c.mu.Lock()
	attacher, ok := c.attachers[aKey]
	delete(c.attachers, aKey)
	c.mu.Unlock()

	if !ok {
		return fmt.Errorf("could not find network attachment for container %s to network %s", containerID, target)
	}

	close(attacher.detachWaitCh)
	return nil
}

// CreateNetwork creates a new cluster managed network.
func (c *Cluster) CreateNetwork(s apitypes.NetworkCreateRequest) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return "", c.errNoManager(state)
	}

	if runconfig.IsPreDefinedNetwork(s.Name) {
		err := fmt.Errorf("%s is a pre-defined network and cannot be created", s.Name)
		return "", apierrors.NewRequestForbiddenError(err)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	networkSpec := convert.BasicNetworkCreateToGRPC(s)
	r, err := state.controlClient.CreateNetwork(ctx, &swarmapi.CreateNetworkRequest{Spec: &networkSpec})
	if err != nil {
		return "", err
	}

	return r.Network.ID, nil
}

// RemoveNetwork removes a cluster network.
func (c *Cluster) RemoveNetwork(input string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	network, err := getNetwork(ctx, state.controlClient, input)
	if err != nil {
		return err
	}

	_, err = state.controlClient.RemoveNetwork(ctx, &swarmapi.RemoveNetworkRequest{NetworkID: network.ID})
	return err
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
				err = fmt.Errorf("The network %s cannot be used with services. Only networks scoped to the swarm can be used, such as those created with the overlay driver.", ln.Name())
				return apierrors.NewRequestForbiddenError(err)
			}
			return err
		}
		networks[i].Target = apiNetwork.ID
	}
	return nil
}

// Cleanup stops active swarm node. This is run before daemon shutdown.
func (c *Cluster) Cleanup() {
	c.controlMutex.Lock()
	defer c.controlMutex.Unlock()

	c.mu.Lock()
	node := c.nr
	if node == nil {
		c.mu.Unlock()
		return
	}
	defer c.mu.Unlock()
	state := c.currentNodeState()
	if state.IsActiveManager() {
		active, reachable, unreachable, err := managerStats(state.controlClient, state.NodeID())
		if err == nil {
			singlenode := active && isLastManager(reachable, unreachable)
			if active && !singlenode && removingManagerCausesLossOfQuorum(reachable, unreachable) {
				logrus.Errorf("Leaving cluster with %v managers left out of %v. Raft quorum will be lost.", reachable-1, reachable+unreachable)
			}
		}
	}
	if err := node.Stop(); err != nil {
		logrus.Errorf("failed to shut down cluster node: %v", err)
		signal.DumpStacks("")
	}
	c.nr = nil
}

func managerStats(client swarmapi.ControlClient, currentNodeID string) (current bool, reachable int, unreachable int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	nodes, err := client.ListNodes(ctx, &swarmapi.ListNodesRequest{})
	if err != nil {
		return false, 0, 0, err
	}
	for _, n := range nodes.Nodes {
		if n.ManagerStatus != nil {
			if n.ManagerStatus.Reachability == swarmapi.RaftMemberStatus_REACHABLE {
				reachable++
				if n.ID == currentNodeID {
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

	if req.Spec.Annotations.Name == "" {
		req.Spec.Annotations.Name = "default"
	} else if req.Spec.Annotations.Name != "default" {
		return errors.New(`swarm spec must be named "default"`)
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
		return errors.New("at least 1 RemoteAddr is required to join")
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
		return addr, errors.New("invalid empty address")
	}
	newaddr, err := opts.ParseTCPAddr(addr, defaultAddr)
	if err != nil {
		return addr, nil
	}
	return strings.TrimPrefix(newaddr, "tcp://"), nil
}

func initClusterSpec(node *swarmnode.Node, spec types.Spec) error {
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
					return errors.New("empty list of clusters was returned")
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

func detectLockedError(err error) error {
	if err == swarmnode.ErrInvalidUnlockKey {
		return errors.WithStack(errSwarmLocked)
	}
	return err
}
