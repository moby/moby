package dispatcher

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/docker/go-events"
	"github.com/docker/go-metrics"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/equality"
	"github.com/docker/swarmkit/ca"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/drivers"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"github.com/docker/swarmkit/remotes"
	"github.com/docker/swarmkit/watch"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/transport"
)

const (
	// DefaultHeartBeatPeriod is used for setting default value in cluster config
	// and in case if cluster config is missing.
	DefaultHeartBeatPeriod       = 5 * time.Second
	defaultHeartBeatEpsilon      = 500 * time.Millisecond
	defaultGracePeriodMultiplier = 3
	defaultRateLimitPeriod       = 8 * time.Second

	// maxBatchItems is the threshold of queued writes that should
	// trigger an actual transaction to commit them to the shared store.
	maxBatchItems = 10000

	// maxBatchInterval needs to strike a balance between keeping
	// latency low, and realizing opportunities to combine many writes
	// into a single transaction. A fraction of a second feels about
	// right.
	maxBatchInterval = 100 * time.Millisecond

	modificationBatchLimit = 100
	batchingWaitTime       = 100 * time.Millisecond

	// defaultNodeDownPeriod specifies the default time period we
	// wait before moving tasks assigned to down nodes to ORPHANED
	// state.
	defaultNodeDownPeriod = 24 * time.Hour
)

var (
	// ErrNodeAlreadyRegistered returned if node with same ID was already
	// registered with this dispatcher.
	ErrNodeAlreadyRegistered = errors.New("node already registered")
	// ErrNodeNotRegistered returned if node with such ID wasn't registered
	// with this dispatcher.
	ErrNodeNotRegistered = errors.New("node not registered")
	// ErrSessionInvalid returned when the session in use is no longer valid.
	// The node should re-register and start a new session.
	ErrSessionInvalid = errors.New("session invalid")
	// ErrNodeNotFound returned when the Node doesn't exist in raft.
	ErrNodeNotFound = errors.New("node not found")

	// Scheduling delay timer.
	schedulingDelayTimer metrics.Timer
)

func init() {
	ns := metrics.NewNamespace("swarm", "dispatcher", nil)
	schedulingDelayTimer = ns.NewTimer("scheduling_delay",
		"Scheduling delay is the time a task takes to go from NEW to RUNNING state.")
	metrics.Register(ns)
}

// Config is configuration for Dispatcher. For default you should use
// DefaultConfig.
type Config struct {
	HeartbeatPeriod  time.Duration
	HeartbeatEpsilon time.Duration
	// RateLimitPeriod specifies how often node with same ID can try to register
	// new session.
	RateLimitPeriod       time.Duration
	GracePeriodMultiplier int
}

// DefaultConfig returns default config for Dispatcher.
func DefaultConfig() *Config {
	return &Config{
		HeartbeatPeriod:       DefaultHeartBeatPeriod,
		HeartbeatEpsilon:      defaultHeartBeatEpsilon,
		RateLimitPeriod:       defaultRateLimitPeriod,
		GracePeriodMultiplier: defaultGracePeriodMultiplier,
	}
}

// Cluster is interface which represent raft cluster. manager/state/raft.Node
// is implements it. This interface needed only for easier unit-testing.
type Cluster interface {
	GetMemberlist() map[uint64]*api.RaftMember
	SubscribePeers() (chan events.Event, func())
	MemoryStore() *store.MemoryStore
}

// nodeUpdate provides a new status and/or description to apply to a node
// object.
type nodeUpdate struct {
	status      *api.NodeStatus
	description *api.NodeDescription
}

// clusterUpdate is an object that stores an update to the cluster that should trigger
// a new session message.  These are pointers to indicate the difference between
// "there is no update" and "update this to nil"
type clusterUpdate struct {
	managerUpdate      *[]*api.WeightedPeer
	bootstrapKeyUpdate *[]*api.EncryptionKey
	rootCAUpdate       *[]byte
}

// Dispatcher is responsible for dispatching tasks and tracking agent health.
type Dispatcher struct {
	// mu is a lock to provide  mutually exclusive access to dispatcher fields
	// e.g. lastSeenManagers, networkBootstrapKeys, lastSeenRootCert etc.
	mu sync.Mutex
	// shutdownWait is used by stop() to wait for existing operations to finish.
	shutdownWait sync.WaitGroup

	nodes                *nodeStore
	store                *store.MemoryStore
	lastSeenManagers     []*api.WeightedPeer
	networkBootstrapKeys []*api.EncryptionKey
	lastSeenRootCert     []byte
	config               *Config
	cluster              Cluster
	ctx                  context.Context
	cancel               context.CancelFunc
	clusterUpdateQueue   *watch.Queue
	dp                   *drivers.DriverProvider
	securityConfig       *ca.SecurityConfig

	taskUpdates     map[string]*api.TaskStatus // indexed by task ID
	taskUpdatesLock sync.Mutex

	nodeUpdates     map[string]nodeUpdate // indexed by node ID
	nodeUpdatesLock sync.Mutex

	downNodes *nodeStore

	processUpdatesTrigger chan struct{}

	// for waiting for the next task/node batch update
	processUpdatesLock sync.Mutex
	processUpdatesCond *sync.Cond
}

// New returns Dispatcher with cluster interface(usually raft.Node).
func New(cluster Cluster, c *Config, dp *drivers.DriverProvider, securityConfig *ca.SecurityConfig) *Dispatcher {
	d := &Dispatcher{
		dp:                    dp,
		nodes:                 newNodeStore(c.HeartbeatPeriod, c.HeartbeatEpsilon, c.GracePeriodMultiplier, c.RateLimitPeriod),
		downNodes:             newNodeStore(defaultNodeDownPeriod, 0, 1, 0),
		store:                 cluster.MemoryStore(),
		cluster:               cluster,
		processUpdatesTrigger: make(chan struct{}, 1),
		config:                c,
		securityConfig:        securityConfig,
	}

	d.processUpdatesCond = sync.NewCond(&d.processUpdatesLock)

	return d
}

func getWeightedPeers(cluster Cluster) []*api.WeightedPeer {
	members := cluster.GetMemberlist()
	var mgrs []*api.WeightedPeer
	for _, m := range members {
		mgrs = append(mgrs, &api.WeightedPeer{
			Peer: &api.Peer{
				NodeID: m.NodeID,
				Addr:   m.Addr,
			},

			// TODO(stevvooe): Calculate weight of manager selection based on
			// cluster-level observations, such as number of connections and
			// load.
			Weight: remotes.DefaultObservationWeight,
		})
	}
	return mgrs
}

// Run runs dispatcher tasks which should be run on leader dispatcher.
// Dispatcher can be stopped with cancelling ctx or calling Stop().
func (d *Dispatcher) Run(ctx context.Context) error {
	ctx = log.WithModule(ctx, "dispatcher")
	log.G(ctx).Info("dispatcher starting")

	d.taskUpdatesLock.Lock()
	d.taskUpdates = make(map[string]*api.TaskStatus)
	d.taskUpdatesLock.Unlock()

	d.nodeUpdatesLock.Lock()
	d.nodeUpdates = make(map[string]nodeUpdate)
	d.nodeUpdatesLock.Unlock()

	d.mu.Lock()
	if d.isRunning() {
		d.mu.Unlock()
		return errors.New("dispatcher is already running")
	}
	if err := d.markNodesUnknown(ctx); err != nil {
		log.G(ctx).Errorf(`failed to move all nodes to "unknown" state: %v`, err)
	}
	configWatcher, cancel, err := store.ViewAndWatch(
		d.store,
		func(readTx store.ReadTx) error {
			clusters, err := store.FindClusters(readTx, store.ByName(store.DefaultClusterName))
			if err != nil {
				return err
			}
			if err == nil && len(clusters) == 1 {
				heartbeatPeriod, err := gogotypes.DurationFromProto(clusters[0].Spec.Dispatcher.HeartbeatPeriod)
				if err == nil && heartbeatPeriod > 0 {
					d.config.HeartbeatPeriod = heartbeatPeriod
				}
				if clusters[0].NetworkBootstrapKeys != nil {
					d.networkBootstrapKeys = clusters[0].NetworkBootstrapKeys
				}
				d.lastSeenRootCert = clusters[0].RootCA.CACert
			}
			return nil
		},
		api.EventUpdateCluster{},
	)
	if err != nil {
		d.mu.Unlock()
		return err
	}
	// set queue here to guarantee that Close will close it
	d.clusterUpdateQueue = watch.NewQueue()

	peerWatcher, peerCancel := d.cluster.SubscribePeers()
	defer peerCancel()
	d.lastSeenManagers = getWeightedPeers(d.cluster)

	defer cancel()
	d.ctx, d.cancel = context.WithCancel(ctx)
	ctx = d.ctx

	// If Stop() is called, it should wait
	// for Run() to complete.
	d.shutdownWait.Add(1)
	defer d.shutdownWait.Done()
	d.mu.Unlock()

	publishManagers := func(peers []*api.Peer) {
		var mgrs []*api.WeightedPeer
		for _, p := range peers {
			mgrs = append(mgrs, &api.WeightedPeer{
				Peer:   p,
				Weight: remotes.DefaultObservationWeight,
			})
		}
		d.mu.Lock()
		d.lastSeenManagers = mgrs
		d.mu.Unlock()
		d.clusterUpdateQueue.Publish(clusterUpdate{managerUpdate: &mgrs})
	}

	batchTimer := time.NewTimer(maxBatchInterval)
	defer batchTimer.Stop()

	for {
		select {
		case ev := <-peerWatcher:
			publishManagers(ev.([]*api.Peer))
		case <-d.processUpdatesTrigger:
			d.processUpdates(ctx)
			batchTimer.Reset(maxBatchInterval)
		case <-batchTimer.C:
			d.processUpdates(ctx)
			batchTimer.Reset(maxBatchInterval)
		case v := <-configWatcher:
			cluster := v.(api.EventUpdateCluster)
			d.mu.Lock()
			if cluster.Cluster.Spec.Dispatcher.HeartbeatPeriod != nil {
				// ignore error, since Spec has passed validation before
				heartbeatPeriod, _ := gogotypes.DurationFromProto(cluster.Cluster.Spec.Dispatcher.HeartbeatPeriod)
				if heartbeatPeriod != d.config.HeartbeatPeriod {
					// only call d.nodes.updatePeriod when heartbeatPeriod changes
					d.config.HeartbeatPeriod = heartbeatPeriod
					d.nodes.updatePeriod(d.config.HeartbeatPeriod, d.config.HeartbeatEpsilon, d.config.GracePeriodMultiplier)
				}
			}
			d.lastSeenRootCert = cluster.Cluster.RootCA.CACert
			d.networkBootstrapKeys = cluster.Cluster.NetworkBootstrapKeys
			d.mu.Unlock()
			d.clusterUpdateQueue.Publish(clusterUpdate{
				bootstrapKeyUpdate: &cluster.Cluster.NetworkBootstrapKeys,
				rootCAUpdate:       &cluster.Cluster.RootCA.CACert,
			})
		case <-ctx.Done():
			return nil
		}
	}
}

// Stop stops dispatcher and closes all grpc streams.
func (d *Dispatcher) Stop() error {
	d.mu.Lock()
	if !d.isRunning() {
		d.mu.Unlock()
		return errors.New("dispatcher is already stopped")
	}

	// Cancel dispatcher context.
	// This should also close the the streams in Tasks(), Assignments().
	d.cancel()
	d.mu.Unlock()

	// Wait for the RPCs that are in-progress to finish.
	d.shutdownWait.Wait()

	d.nodes.Clean()

	d.processUpdatesLock.Lock()
	// In case there are any waiters. There is no chance of any starting
	// after this point, because they check if the context is canceled
	// before waiting.
	d.processUpdatesCond.Broadcast()
	d.processUpdatesLock.Unlock()

	d.clusterUpdateQueue.Close()
	return nil
}

func (d *Dispatcher) isRunningLocked() (context.Context, error) {
	d.mu.Lock()
	if !d.isRunning() {
		d.mu.Unlock()
		return nil, status.Errorf(codes.Aborted, "dispatcher is stopped")
	}
	ctx := d.ctx
	d.mu.Unlock()
	return ctx, nil
}

func (d *Dispatcher) markNodesUnknown(ctx context.Context) error {
	log := log.G(ctx).WithField("method", "(*Dispatcher).markNodesUnknown")
	var nodes []*api.Node
	var err error
	d.store.View(func(tx store.ReadTx) {
		nodes, err = store.FindNodes(tx, store.All)
	})
	if err != nil {
		return errors.Wrap(err, "failed to get list of nodes")
	}
	err = d.store.Batch(func(batch *store.Batch) error {
		for _, n := range nodes {
			err := batch.Update(func(tx store.Tx) error {
				// check if node is still here
				node := store.GetNode(tx, n.ID)
				if node == nil {
					return nil
				}
				// do not try to resurrect down nodes
				if node.Status.State == api.NodeStatus_DOWN {
					nodeCopy := node
					expireFunc := func() {
						log.Infof("moving tasks to orphaned state for node: %s", nodeCopy.ID)
						if err := d.moveTasksToOrphaned(nodeCopy.ID); err != nil {
							log.WithError(err).Errorf(`failed to move all tasks for node %s to "ORPHANED" state`, node.ID)
						}

						d.downNodes.Delete(nodeCopy.ID)
					}

					log.Infof(`node %s was found to be down when marking unknown on dispatcher start`, node.ID)
					d.downNodes.Add(nodeCopy, expireFunc)
					return nil
				}

				node.Status.State = api.NodeStatus_UNKNOWN
				node.Status.Message = `Node moved to "unknown" state due to leadership change in cluster`

				nodeID := node.ID

				expireFunc := func() {
					log := log.WithField("node", nodeID)
					log.Info(`heartbeat expiration for node %s in state "unknown"`, nodeID)
					if err := d.markNodeNotReady(nodeID, api.NodeStatus_DOWN, `heartbeat failure for node in "unknown" state`); err != nil {
						log.WithError(err).Error(`failed deregistering node after heartbeat expiration for node in "unknown" state`)
					}
				}
				if err := d.nodes.AddUnknown(node, expireFunc); err != nil {
					return errors.Wrapf(err, `adding node %s in "unknown" state to node store failed`, nodeID)
				}
				if err := store.UpdateNode(tx, node); err != nil {
					return errors.Wrapf(err, "update for node %s failed", nodeID)
				}
				return nil
			})
			if err != nil {
				log.WithField("node", n.ID).WithError(err).Error(`failed to move node to "unknown" state`)
			}
		}
		return nil
	})
	return err
}

func (d *Dispatcher) isRunning() bool {
	if d.ctx == nil {
		return false
	}
	select {
	case <-d.ctx.Done():
		return false
	default:
	}
	return true
}

// markNodeReady updates the description of a node, updates its address, and sets status to READY
// this is used during registration when a new node description is provided
// and during node updates when the node description changes
func (d *Dispatcher) markNodeReady(ctx context.Context, nodeID string, description *api.NodeDescription, addr string) error {
	d.nodeUpdatesLock.Lock()
	d.nodeUpdates[nodeID] = nodeUpdate{
		status: &api.NodeStatus{
			State: api.NodeStatus_READY,
			Addr:  addr,
		},
		description: description,
	}
	numUpdates := len(d.nodeUpdates)
	d.nodeUpdatesLock.Unlock()

	// Node is marked ready. Remove the node from down nodes if it
	// is there.
	d.downNodes.Delete(nodeID)

	if numUpdates >= maxBatchItems {
		select {
		case d.processUpdatesTrigger <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}

	}

	// Wait until the node update batch happens before unblocking register.
	d.processUpdatesLock.Lock()
	defer d.processUpdatesLock.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	d.processUpdatesCond.Wait()

	return nil
}

// gets the node IP from the context of a grpc call
func nodeIPFromContext(ctx context.Context) (string, error) {
	nodeInfo, err := ca.RemoteNode(ctx)
	if err != nil {
		return "", err
	}
	addr, _, err := net.SplitHostPort(nodeInfo.RemoteAddr)
	if err != nil {
		return "", errors.Wrap(err, "unable to get ip from addr:port")
	}
	return addr, nil
}

// register is used for registration of node with particular dispatcher.
func (d *Dispatcher) register(ctx context.Context, nodeID string, description *api.NodeDescription) (string, error) {
	dctx, err := d.isRunningLocked()
	if err != nil {
		return "", err
	}

	logLocal := log.G(ctx).WithField("method", "(*Dispatcher).register")

	if err := d.nodes.CheckRateLimit(nodeID); err != nil {
		return "", err
	}

	// TODO(stevvooe): Validate node specification.
	var node *api.Node
	d.store.View(func(tx store.ReadTx) {
		node = store.GetNode(tx, nodeID)
	})
	if node == nil {
		return "", ErrNodeNotFound
	}

	addr, err := nodeIPFromContext(ctx)
	if err != nil {
		logLocal.WithError(err).Debug("failed to get remote node IP")
	}

	if err := d.markNodeReady(dctx, nodeID, description, addr); err != nil {
		return "", err
	}

	expireFunc := func() {
		log.G(ctx).Debug("heartbeat expiration for worker %s, setting worker status to NodeStatus_DOWN ", nodeID)
		if err := d.markNodeNotReady(nodeID, api.NodeStatus_DOWN, "heartbeat failure"); err != nil {
			log.G(ctx).WithError(err).Errorf("failed deregistering node after heartbeat expiration")
		}
	}

	rn := d.nodes.Add(node, expireFunc)
	logLocal.Infof("worker %s was successfully registered", nodeID)

	// NOTE(stevvooe): We need be a little careful with re-registration. The
	// current implementation just matches the node id and then gives away the
	// sessionID. If we ever want to use sessionID as a secret, which we may
	// want to, this is giving away the keys to the kitchen.
	//
	// The right behavior is going to be informed by identity. Basically, each
	// time a node registers, we invalidate the session and issue a new
	// session, once identity is proven. This will cause misbehaved agents to
	// be kicked when multiple connections are made.
	return rn.SessionID, nil
}

// UpdateTaskStatus updates status of task. Node should send such updates
// on every status change of its tasks.
func (d *Dispatcher) UpdateTaskStatus(ctx context.Context, r *api.UpdateTaskStatusRequest) (*api.UpdateTaskStatusResponse, error) {
	// shutdownWait.Add() followed by isRunning() to ensures that
	// if this rpc sees the dispatcher running,
	// it will already have called Add() on the shutdownWait wait,
	// which ensures that Stop() will wait for this rpc to complete.
	// Note that Stop() first does Dispatcher.ctx.cancel() followed by
	// shutdownWait.Wait() to make sure new rpc's don't start before waiting
	// for existing ones to finish.
	d.shutdownWait.Add(1)
	defer d.shutdownWait.Done()

	dctx, err := d.isRunningLocked()
	if err != nil {
		return nil, err
	}

	nodeInfo, err := ca.RemoteNode(ctx)
	if err != nil {
		return nil, err
	}
	nodeID := nodeInfo.NodeID
	fields := logrus.Fields{
		"node.id":      nodeID,
		"node.session": r.SessionID,
		"method":       "(*Dispatcher).UpdateTaskStatus",
	}
	if nodeInfo.ForwardedBy != nil {
		fields["forwarder.id"] = nodeInfo.ForwardedBy.NodeID
	}
	log := log.G(ctx).WithFields(fields)

	if _, err := d.nodes.GetWithSession(nodeID, r.SessionID); err != nil {
		return nil, err
	}

	validTaskUpdates := make([]*api.UpdateTaskStatusRequest_TaskStatusUpdate, 0, len(r.Updates))

	// Validate task updates
	for _, u := range r.Updates {
		if u.Status == nil {
			log.WithField("task.id", u.TaskID).Warn("task report has nil status")
			continue
		}

		var t *api.Task
		d.store.View(func(tx store.ReadTx) {
			t = store.GetTask(tx, u.TaskID)
		})
		if t == nil {
			// Task may have been deleted
			log.WithField("task.id", u.TaskID).Debug("cannot find target task in store")
			continue
		}

		if t.NodeID != nodeID {
			err := status.Errorf(codes.PermissionDenied, "cannot update a task not assigned this node")
			log.WithField("task.id", u.TaskID).Error(err)
			return nil, err
		}

		validTaskUpdates = append(validTaskUpdates, u)
	}

	d.taskUpdatesLock.Lock()
	// Enqueue task updates
	for _, u := range validTaskUpdates {
		d.taskUpdates[u.TaskID] = u.Status
	}

	numUpdates := len(d.taskUpdates)
	d.taskUpdatesLock.Unlock()

	if numUpdates >= maxBatchItems {
		select {
		case d.processUpdatesTrigger <- struct{}{}:
		case <-dctx.Done():
		}
	}
	return nil, nil
}

func (d *Dispatcher) processUpdates(ctx context.Context) {
	var (
		taskUpdates map[string]*api.TaskStatus
		nodeUpdates map[string]nodeUpdate
	)
	d.taskUpdatesLock.Lock()
	if len(d.taskUpdates) != 0 {
		taskUpdates = d.taskUpdates
		d.taskUpdates = make(map[string]*api.TaskStatus)
	}
	d.taskUpdatesLock.Unlock()

	d.nodeUpdatesLock.Lock()
	if len(d.nodeUpdates) != 0 {
		nodeUpdates = d.nodeUpdates
		d.nodeUpdates = make(map[string]nodeUpdate)
	}
	d.nodeUpdatesLock.Unlock()

	if len(taskUpdates) == 0 && len(nodeUpdates) == 0 {
		return
	}

	log := log.G(ctx).WithFields(logrus.Fields{
		"method": "(*Dispatcher).processUpdates",
	})

	err := d.store.Batch(func(batch *store.Batch) error {
		for taskID, status := range taskUpdates {
			err := batch.Update(func(tx store.Tx) error {
				logger := log.WithField("task.id", taskID)
				task := store.GetTask(tx, taskID)
				if task == nil {
					// Task may have been deleted
					logger.Debug("cannot find target task in store")
					return nil
				}

				logger = logger.WithField("state.transition", fmt.Sprintf("%v->%v", task.Status.State, status.State))

				if task.Status == *status {
					logger.Debug("task status identical, ignoring")
					return nil
				}

				if task.Status.State > status.State {
					logger.Debug("task status invalid transition")
					return nil
				}

				// Update scheduling delay metric for running tasks.
				// We use the status update time on the leader to calculate the scheduling delay.
				// Because of this, the recorded scheduling delay will be an overestimate and include
				// the network delay between the worker and the leader.
				// This is not ideal, but its a known overestimation, rather than using the status update time
				// from the worker node, which may cause unknown incorrect results due to possible clock skew.
				if status.State == api.TaskStateRunning {
					start := time.Unix(status.AppliedAt.GetSeconds(), int64(status.AppliedAt.GetNanos()))
					schedulingDelayTimer.UpdateSince(start)
				}

				task.Status = *status
				task.Status.AppliedBy = d.securityConfig.ClientTLSCreds.NodeID()
				task.Status.AppliedAt = ptypes.MustTimestampProto(time.Now())
				if err := store.UpdateTask(tx, task); err != nil {
					logger.WithError(err).Error("failed to update task status")
					return nil
				}
				logger.Debug("dispatcher committed status update to store")
				return nil
			})
			if err != nil {
				log.WithError(err).Error("dispatcher task update transaction failed")
			}
		}

		for nodeID, nodeUpdate := range nodeUpdates {
			err := batch.Update(func(tx store.Tx) error {
				logger := log.WithField("node.id", nodeID)
				node := store.GetNode(tx, nodeID)
				if node == nil {
					logger.Errorf("node unavailable")
					return nil
				}

				if nodeUpdate.status != nil {
					node.Status.State = nodeUpdate.status.State
					node.Status.Message = nodeUpdate.status.Message
					if nodeUpdate.status.Addr != "" {
						node.Status.Addr = nodeUpdate.status.Addr
					}
				}
				if nodeUpdate.description != nil {
					node.Description = nodeUpdate.description
				}

				if err := store.UpdateNode(tx, node); err != nil {
					logger.WithError(err).Error("failed to update node status")
					return nil
				}
				logger.Debug("node status updated")
				return nil
			})
			if err != nil {
				log.WithError(err).Error("dispatcher node update transaction failed")
			}
		}

		return nil
	})
	if err != nil {
		log.WithError(err).Error("dispatcher batch failed")
	}

	d.processUpdatesCond.Broadcast()
}

// Tasks is a stream of tasks state for node. Each message contains full list
// of tasks which should be run on node, if task is not present in that list,
// it should be terminated.
func (d *Dispatcher) Tasks(r *api.TasksRequest, stream api.Dispatcher_TasksServer) error {
	// shutdownWait.Add() followed by isRunning() to ensures that
	// if this rpc sees the dispatcher running,
	// it will already have called Add() on the shutdownWait wait,
	// which ensures that Stop() will wait for this rpc to complete.
	// Note that Stop() first does Dispatcher.ctx.cancel() followed by
	// shutdownWait.Wait() to make sure new rpc's don't start before waiting
	// for existing ones to finish.
	d.shutdownWait.Add(1)
	defer d.shutdownWait.Done()

	dctx, err := d.isRunningLocked()
	if err != nil {
		return err
	}

	nodeInfo, err := ca.RemoteNode(stream.Context())
	if err != nil {
		return err
	}
	nodeID := nodeInfo.NodeID

	fields := logrus.Fields{
		"node.id":      nodeID,
		"node.session": r.SessionID,
		"method":       "(*Dispatcher).Tasks",
	}
	if nodeInfo.ForwardedBy != nil {
		fields["forwarder.id"] = nodeInfo.ForwardedBy.NodeID
	}
	log.G(stream.Context()).WithFields(fields).Debug("")

	if _, err = d.nodes.GetWithSession(nodeID, r.SessionID); err != nil {
		return err
	}

	tasksMap := make(map[string]*api.Task)
	nodeTasks, cancel, err := store.ViewAndWatch(
		d.store,
		func(readTx store.ReadTx) error {
			tasks, err := store.FindTasks(readTx, store.ByNodeID(nodeID))
			if err != nil {
				return err
			}
			for _, t := range tasks {
				tasksMap[t.ID] = t
			}
			return nil
		},
		api.EventCreateTask{Task: &api.Task{NodeID: nodeID},
			Checks: []api.TaskCheckFunc{api.TaskCheckNodeID}},
		api.EventUpdateTask{Task: &api.Task{NodeID: nodeID},
			Checks: []api.TaskCheckFunc{api.TaskCheckNodeID}},
		api.EventDeleteTask{Task: &api.Task{NodeID: nodeID},
			Checks: []api.TaskCheckFunc{api.TaskCheckNodeID}},
	)
	if err != nil {
		return err
	}
	defer cancel()

	for {
		if _, err := d.nodes.GetWithSession(nodeID, r.SessionID); err != nil {
			return err
		}

		var tasks []*api.Task
		for _, t := range tasksMap {
			// dispatcher only sends tasks that have been assigned to a node
			if t != nil && t.Status.State >= api.TaskStateAssigned {
				tasks = append(tasks, t)
			}
		}

		if err := stream.Send(&api.TasksMessage{Tasks: tasks}); err != nil {
			return err
		}

		// bursty events should be processed in batches and sent out snapshot
		var (
			modificationCnt int
			batchingTimer   *time.Timer
			batchingTimeout <-chan time.Time
		)

	batchingLoop:
		for modificationCnt < modificationBatchLimit {
			select {
			case event := <-nodeTasks:
				switch v := event.(type) {
				case api.EventCreateTask:
					tasksMap[v.Task.ID] = v.Task
					modificationCnt++
				case api.EventUpdateTask:
					if oldTask, exists := tasksMap[v.Task.ID]; exists {
						// States ASSIGNED and below are set by the orchestrator/scheduler,
						// not the agent, so tasks in these states need to be sent to the
						// agent even if nothing else has changed.
						if equality.TasksEqualStable(oldTask, v.Task) && v.Task.Status.State > api.TaskStateAssigned {
							// this update should not trigger action at agent
							tasksMap[v.Task.ID] = v.Task
							continue
						}
					}
					tasksMap[v.Task.ID] = v.Task
					modificationCnt++
				case api.EventDeleteTask:
					delete(tasksMap, v.Task.ID)
					modificationCnt++
				}
				if batchingTimer != nil {
					batchingTimer.Reset(batchingWaitTime)
				} else {
					batchingTimer = time.NewTimer(batchingWaitTime)
					batchingTimeout = batchingTimer.C
				}
			case <-batchingTimeout:
				break batchingLoop
			case <-stream.Context().Done():
				return stream.Context().Err()
			case <-dctx.Done():
				return dctx.Err()
			}
		}

		if batchingTimer != nil {
			batchingTimer.Stop()
		}
	}
}

// Assignments is a stream of assignments for a node. Each message contains
// either full list of tasks and secrets for the node, or an incremental update.
func (d *Dispatcher) Assignments(r *api.AssignmentsRequest, stream api.Dispatcher_AssignmentsServer) error {
	// shutdownWait.Add() followed by isRunning() to ensures that
	// if this rpc sees the dispatcher running,
	// it will already have called Add() on the shutdownWait wait,
	// which ensures that Stop() will wait for this rpc to complete.
	// Note that Stop() first does Dispatcher.ctx.cancel() followed by
	// shutdownWait.Wait() to make sure new rpc's don't start before waiting
	// for existing ones to finish.
	d.shutdownWait.Add(1)
	defer d.shutdownWait.Done()

	dctx, err := d.isRunningLocked()
	if err != nil {
		return err
	}

	nodeInfo, err := ca.RemoteNode(stream.Context())
	if err != nil {
		return err
	}
	nodeID := nodeInfo.NodeID

	fields := logrus.Fields{
		"node.id":      nodeID,
		"node.session": r.SessionID,
		"method":       "(*Dispatcher).Assignments",
	}
	if nodeInfo.ForwardedBy != nil {
		fields["forwarder.id"] = nodeInfo.ForwardedBy.NodeID
	}
	log := log.G(stream.Context()).WithFields(fields)
	log.Debug("")

	if _, err = d.nodes.GetWithSession(nodeID, r.SessionID); err != nil {
		return err
	}

	var (
		sequence    int64
		appliesTo   string
		assignments = newAssignmentSet(log, d.dp)
	)

	sendMessage := func(msg api.AssignmentsMessage, assignmentType api.AssignmentsMessage_Type) error {
		sequence++
		msg.AppliesTo = appliesTo
		msg.ResultsIn = strconv.FormatInt(sequence, 10)
		appliesTo = msg.ResultsIn
		msg.Type = assignmentType

		return stream.Send(&msg)
	}

	// TODO(aaronl): Also send node secrets that should be exposed to
	// this node.
	nodeTasks, cancel, err := store.ViewAndWatch(
		d.store,
		func(readTx store.ReadTx) error {
			tasks, err := store.FindTasks(readTx, store.ByNodeID(nodeID))
			if err != nil {
				return err
			}

			for _, t := range tasks {
				assignments.addOrUpdateTask(readTx, t)
			}

			return nil
		},
		api.EventUpdateTask{Task: &api.Task{NodeID: nodeID},
			Checks: []api.TaskCheckFunc{api.TaskCheckNodeID}},
		api.EventDeleteTask{Task: &api.Task{NodeID: nodeID},
			Checks: []api.TaskCheckFunc{api.TaskCheckNodeID}},
	)
	if err != nil {
		return err
	}
	defer cancel()

	if err := sendMessage(assignments.message(), api.AssignmentsMessage_COMPLETE); err != nil {
		return err
	}

	for {
		// Check for session expiration
		if _, err := d.nodes.GetWithSession(nodeID, r.SessionID); err != nil {
			return err
		}

		// bursty events should be processed in batches and sent out together
		var (
			modificationCnt int
			batchingTimer   *time.Timer
			batchingTimeout <-chan time.Time
		)

		oneModification := func() {
			modificationCnt++

			if batchingTimer != nil {
				batchingTimer.Reset(batchingWaitTime)
			} else {
				batchingTimer = time.NewTimer(batchingWaitTime)
				batchingTimeout = batchingTimer.C
			}
		}

		// The batching loop waits for 50 ms after the most recent
		// change, or until modificationBatchLimit is reached. The
		// worst case latency is modificationBatchLimit * batchingWaitTime,
		// which is 10 seconds.
	batchingLoop:
		for modificationCnt < modificationBatchLimit {
			select {
			case event := <-nodeTasks:
				switch v := event.(type) {
				// We don't monitor EventCreateTask because tasks are
				// never created in the ASSIGNED state. First tasks are
				// created by the orchestrator, then the scheduler moves
				// them to ASSIGNED. If this ever changes, we will need
				// to monitor task creations as well.
				case api.EventUpdateTask:
					d.store.View(func(readTx store.ReadTx) {
						if assignments.addOrUpdateTask(readTx, v.Task) {
							oneModification()
						}
					})
				case api.EventDeleteTask:
					if assignments.removeTask(v.Task) {
						oneModification()
					}
					// TODO(aaronl): For node secrets, we'll need to handle
					// EventCreateSecret.
				}
			case <-batchingTimeout:
				break batchingLoop
			case <-stream.Context().Done():
				return stream.Context().Err()
			case <-dctx.Done():
				return dctx.Err()
			}
		}

		if batchingTimer != nil {
			batchingTimer.Stop()
		}

		if modificationCnt > 0 {
			if err := sendMessage(assignments.message(), api.AssignmentsMessage_INCREMENTAL); err != nil {
				return err
			}
		}
	}
}

func (d *Dispatcher) moveTasksToOrphaned(nodeID string) error {
	err := d.store.Batch(func(batch *store.Batch) error {
		var (
			tasks []*api.Task
			err   error
		)

		d.store.View(func(tx store.ReadTx) {
			tasks, err = store.FindTasks(tx, store.ByNodeID(nodeID))
		})
		if err != nil {
			return err
		}

		for _, task := range tasks {
			// Tasks running on an unreachable node need to be marked as
			// orphaned since we have no idea whether the task is still running
			// or not.
			//
			// This only applies for tasks that could have made progress since
			// the agent became unreachable (assigned<->running)
			//
			// Tasks in a final state (e.g. rejected) *cannot* have made
			// progress, therefore there's no point in marking them as orphaned
			if task.Status.State >= api.TaskStateAssigned && task.Status.State <= api.TaskStateRunning {
				task.Status.State = api.TaskStateOrphaned
			}

			if err := batch.Update(func(tx store.Tx) error {
				err := store.UpdateTask(tx, task)
				if err != nil {
					return err
				}

				return nil
			}); err != nil {
				return err
			}

		}

		return nil
	})

	return err
}

// markNodeNotReady sets the node state to some state other than READY
func (d *Dispatcher) markNodeNotReady(id string, state api.NodeStatus_State, message string) error {
	logLocal := log.G(d.ctx).WithField("method", "(*Dispatcher).markNodeNotReady")

	dctx, err := d.isRunningLocked()
	if err != nil {
		return err
	}

	// Node is down. Add it to down nodes so that we can keep
	// track of tasks assigned to the node.
	var node *api.Node
	d.store.View(func(readTx store.ReadTx) {
		node = store.GetNode(readTx, id)
		if node == nil {
			err = fmt.Errorf("could not find node %s while trying to add to down nodes store", id)
		}
	})
	if err != nil {
		return err
	}

	expireFunc := func() {
		log.G(dctx).Debugf(`worker timed-out %s in "down" state, moving all tasks to "ORPHANED" state`, id)
		if err := d.moveTasksToOrphaned(id); err != nil {
			log.G(dctx).WithError(err).Error(`failed to move all tasks to "ORPHANED" state`)
		}

		d.downNodes.Delete(id)
	}

	d.downNodes.Add(node, expireFunc)
	logLocal.Debugf("added node %s to down nodes list", node.ID)

	status := &api.NodeStatus{
		State:   state,
		Message: message,
	}

	d.nodeUpdatesLock.Lock()
	// pluck the description out of nodeUpdates. this protects against a case
	// where a node is marked ready and a description is added, but then the
	// node is immediately marked not ready. this preserves that description
	d.nodeUpdates[id] = nodeUpdate{status: status, description: d.nodeUpdates[id].description}
	numUpdates := len(d.nodeUpdates)
	d.nodeUpdatesLock.Unlock()

	if numUpdates >= maxBatchItems {
		select {
		case d.processUpdatesTrigger <- struct{}{}:
		case <-dctx.Done():
		}
	}

	if rn := d.nodes.Delete(id); rn == nil {
		return errors.Errorf("node %s is not found in local storage", id)
	}
	logLocal.Debugf("deleted node %s from node store", node.ID)

	return nil
}

// Heartbeat is heartbeat method for nodes. It returns new TTL in response.
// Node should send new heartbeat earlier than now + TTL, otherwise it will
// be deregistered from dispatcher and its status will be updated to NodeStatus_DOWN
func (d *Dispatcher) Heartbeat(ctx context.Context, r *api.HeartbeatRequest) (*api.HeartbeatResponse, error) {
	// shutdownWait.Add() followed by isRunning() to ensures that
	// if this rpc sees the dispatcher running,
	// it will already have called Add() on the shutdownWait wait,
	// which ensures that Stop() will wait for this rpc to complete.
	// Note that Stop() first does Dispatcher.ctx.cancel() followed by
	// shutdownWait.Wait() to make sure new rpc's don't start before waiting
	// for existing ones to finish.
	d.shutdownWait.Add(1)
	defer d.shutdownWait.Done()

	// isRunningLocked() is not needed since its OK if
	// the dispatcher context is cancelled while this call is in progress
	// since Stop() which cancels the dispatcher context will wait for
	// Heartbeat() to complete.
	if !d.isRunning() {
		return nil, status.Errorf(codes.Aborted, "dispatcher is stopped")
	}

	nodeInfo, err := ca.RemoteNode(ctx)
	if err != nil {
		return nil, err
	}

	period, err := d.nodes.Heartbeat(nodeInfo.NodeID, r.SessionID)

	log.G(ctx).WithField("method", "(*Dispatcher).Heartbeat").Debugf("received heartbeat from worker %v, expect next heartbeat in %v", nodeInfo, period)
	return &api.HeartbeatResponse{Period: period}, err
}

func (d *Dispatcher) getManagers() []*api.WeightedPeer {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastSeenManagers
}

func (d *Dispatcher) getNetworkBootstrapKeys() []*api.EncryptionKey {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.networkBootstrapKeys
}

func (d *Dispatcher) getRootCACert() []byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastSeenRootCert
}

// Session is a stream which controls agent connection.
// Each message contains list of backup Managers with weights. Also there is
// a special boolean field Disconnect which if true indicates that node should
// reconnect to another Manager immediately.
func (d *Dispatcher) Session(r *api.SessionRequest, stream api.Dispatcher_SessionServer) error {
	// shutdownWait.Add() followed by isRunning() to ensures that
	// if this rpc sees the dispatcher running,
	// it will already have called Add() on the shutdownWait wait,
	// which ensures that Stop() will wait for this rpc to complete.
	// Note that Stop() first does Dispatcher.ctx.cancel() followed by
	// shutdownWait.Wait() to make sure new rpc's don't start before waiting
	// for existing ones to finish.
	d.shutdownWait.Add(1)
	defer d.shutdownWait.Done()

	dctx, err := d.isRunningLocked()
	if err != nil {
		return err
	}

	ctx := stream.Context()
	nodeInfo, err := ca.RemoteNode(ctx)
	if err != nil {
		return err
	}
	nodeID := nodeInfo.NodeID

	var sessionID string
	if _, err := d.nodes.GetWithSession(nodeID, r.SessionID); err != nil {
		// register the node.
		sessionID, err = d.register(ctx, nodeID, r.Description)
		if err != nil {
			return err
		}
	} else {
		sessionID = r.SessionID
		// get the node IP addr
		addr, err := nodeIPFromContext(stream.Context())
		if err != nil {
			log.G(ctx).WithError(err).Debug("failed to get remote node IP")
		}
		// update the node description
		if err := d.markNodeReady(dctx, nodeID, r.Description, addr); err != nil {
			return err
		}
	}

	fields := logrus.Fields{
		"node.id":      nodeID,
		"node.session": sessionID,
		"method":       "(*Dispatcher).Session",
	}
	if nodeInfo.ForwardedBy != nil {
		fields["forwarder.id"] = nodeInfo.ForwardedBy.NodeID
	}
	log := log.G(ctx).WithFields(fields)

	var nodeObj *api.Node
	nodeUpdates, cancel, err := store.ViewAndWatch(d.store, func(readTx store.ReadTx) error {
		nodeObj = store.GetNode(readTx, nodeID)
		return nil
	}, api.EventUpdateNode{Node: &api.Node{ID: nodeID},
		Checks: []api.NodeCheckFunc{api.NodeCheckID}},
	)
	if cancel != nil {
		defer cancel()
	}

	if err != nil {
		log.WithError(err).Error("ViewAndWatch Node failed")
	}

	if _, err = d.nodes.GetWithSession(nodeID, sessionID); err != nil {
		return err
	}

	clusterUpdatesCh, clusterCancel := d.clusterUpdateQueue.Watch()
	defer clusterCancel()

	if err := stream.Send(&api.SessionMessage{
		SessionID:            sessionID,
		Node:                 nodeObj,
		Managers:             d.getManagers(),
		NetworkBootstrapKeys: d.getNetworkBootstrapKeys(),
		RootCA:               d.getRootCACert(),
	}); err != nil {
		return err
	}

	// disconnectNode is a helper forcibly shutdown connection
	disconnectNode := func() error {
		// force disconnect by shutting down the stream.
		transportStream, ok := transport.StreamFromContext(stream.Context())
		if ok {
			// if we have the transport stream, we can signal a disconnect
			// in the client.
			if err := transportStream.ServerTransport().Close(); err != nil {
				log.WithError(err).Error("session end")
			}
		}

		log.Infof("dispatcher session dropped, marking node %s down", nodeID)
		if err := d.markNodeNotReady(nodeID, api.NodeStatus_DISCONNECTED, "node is currently trying to find new manager"); err != nil {
			log.WithError(err).Error("failed to remove node")
		}
		// still return an abort if the transport closure was ineffective.
		return status.Errorf(codes.Aborted, "node must disconnect")
	}

	for {
		// After each message send, we need to check the nodes sessionID hasn't
		// changed. If it has, we will shut down the stream and make the node
		// re-register.
		node, err := d.nodes.GetWithSession(nodeID, sessionID)
		if err != nil {
			return err
		}

		var (
			disconnect bool
			mgrs       []*api.WeightedPeer
			netKeys    []*api.EncryptionKey
			rootCert   []byte
		)

		select {
		case ev := <-clusterUpdatesCh:
			update := ev.(clusterUpdate)
			if update.managerUpdate != nil {
				mgrs = *update.managerUpdate
			}
			if update.bootstrapKeyUpdate != nil {
				netKeys = *update.bootstrapKeyUpdate
			}
			if update.rootCAUpdate != nil {
				rootCert = *update.rootCAUpdate
			}
		case ev := <-nodeUpdates:
			nodeObj = ev.(api.EventUpdateNode).Node
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-node.Disconnect:
			disconnect = true
		case <-dctx.Done():
			disconnect = true
		}
		if mgrs == nil {
			mgrs = d.getManagers()
		}
		if netKeys == nil {
			netKeys = d.getNetworkBootstrapKeys()
		}
		if rootCert == nil {
			rootCert = d.getRootCACert()
		}

		if err := stream.Send(&api.SessionMessage{
			SessionID:            sessionID,
			Node:                 nodeObj,
			Managers:             mgrs,
			NetworkBootstrapKeys: netKeys,
			RootCA:               rootCert,
		}); err != nil {
			return err
		}
		if disconnect {
			return disconnectNode()
		}
	}
}
