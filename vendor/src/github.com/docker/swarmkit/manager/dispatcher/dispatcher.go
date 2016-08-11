package dispatcher

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/transport"

	"github.com/Sirupsen/logrus"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/equality"
	"github.com/docker/swarmkit/ca"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/manager/state/watch"
	"github.com/docker/swarmkit/picker"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"golang.org/x/net/context"
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
)

// Config is configuration for Dispatcher. For default you should use
// DefautConfig.
type Config struct {
	// Addr configures the address the dispatcher reports to agents.
	Addr             string
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

// Cluster is interface which represent raft cluster. mananger/state/raft.Node
// is implenents it. This interface needed only for easier unit-testing.
type Cluster interface {
	GetMemberlist() map[uint64]*api.RaftMember
	MemoryStore() *store.MemoryStore
}

// Dispatcher is responsible for dispatching tasks and tracking agent health.
type Dispatcher struct {
	mu                   sync.Mutex
	addr                 string
	nodes                *nodeStore
	store                *store.MemoryStore
	mgrQueue             *watch.Queue
	lastSeenManagers     []*api.WeightedPeer
	networkBootstrapKeys []*api.EncryptionKey
	keyMgrQueue          *watch.Queue
	config               *Config
	cluster              Cluster
	ctx                  context.Context
	cancel               context.CancelFunc

	taskUpdates     map[string]*api.TaskStatus // indexed by task ID
	taskUpdatesLock sync.Mutex

	processTaskUpdatesTrigger chan struct{}
}

// weightedPeerByNodeID is a sort wrapper for []*api.WeightedPeer
type weightedPeerByNodeID []*api.WeightedPeer

func (b weightedPeerByNodeID) Less(i, j int) bool { return b[i].Peer.NodeID < b[j].Peer.NodeID }

func (b weightedPeerByNodeID) Len() int { return len(b) }

func (b weightedPeerByNodeID) Swap(i, j int) { b[i], b[j] = b[j], b[i] }

// New returns Dispatcher with cluster interface(usually raft.Node).
// NOTE: each handler which does something with raft must add to Dispatcher.wg
func New(cluster Cluster, c *Config) *Dispatcher {
	return &Dispatcher{
		addr:                      c.Addr,
		nodes:                     newNodeStore(c.HeartbeatPeriod, c.HeartbeatEpsilon, c.GracePeriodMultiplier, c.RateLimitPeriod),
		store:                     cluster.MemoryStore(),
		cluster:                   cluster,
		mgrQueue:                  watch.NewQueue(16),
		keyMgrQueue:               watch.NewQueue(16),
		taskUpdates:               make(map[string]*api.TaskStatus),
		processTaskUpdatesTrigger: make(chan struct{}, 1),
		config: c,
	}
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
			Weight: picker.DefaultObservationWeight,
		})
	}
	return mgrs
}

// Run runs dispatcher tasks which should be run on leader dispatcher.
// Dispatcher can be stopped with cancelling ctx or calling Stop().
func (d *Dispatcher) Run(ctx context.Context) error {
	d.mu.Lock()
	if d.isRunning() {
		d.mu.Unlock()
		return fmt.Errorf("dispatcher is already running")
	}
	logger := log.G(ctx).WithField("module", "dispatcher")
	ctx = log.WithLogger(ctx, logger)
	if err := d.markNodesUnknown(ctx); err != nil {
		logger.Errorf(`failed to move all nodes to "unknown" state: %v`, err)
	}
	configWatcher, cancel, err := store.ViewAndWatch(
		d.store,
		func(readTx store.ReadTx) error {
			clusters, err := store.FindClusters(readTx, store.ByName(store.DefaultClusterName))
			if err != nil {
				return err
			}
			if err == nil && len(clusters) == 1 {
				heartbeatPeriod, err := ptypes.Duration(clusters[0].Spec.Dispatcher.HeartbeatPeriod)
				if err == nil && heartbeatPeriod > 0 {
					d.config.HeartbeatPeriod = heartbeatPeriod
				}
				if clusters[0].NetworkBootstrapKeys != nil {
					d.networkBootstrapKeys = clusters[0].NetworkBootstrapKeys
				}
			}
			return nil
		},
		state.EventUpdateCluster{},
	)
	if err != nil {
		d.mu.Unlock()
		return err
	}
	defer cancel()
	d.ctx, d.cancel = context.WithCancel(ctx)
	d.mu.Unlock()

	publishManagers := func() {
		mgrs := getWeightedPeers(d.cluster)
		sort.Sort(weightedPeerByNodeID(mgrs))
		d.mu.Lock()
		if reflect.DeepEqual(mgrs, d.lastSeenManagers) {
			d.mu.Unlock()
			return
		}
		d.lastSeenManagers = mgrs
		d.mu.Unlock()
		d.mgrQueue.Publish(mgrs)
	}

	publishManagers()
	publishTicker := time.NewTicker(1 * time.Second)
	defer publishTicker.Stop()

	batchTimer := time.NewTimer(maxBatchInterval)
	defer batchTimer.Stop()

	for {
		select {
		case <-publishTicker.C:
			publishManagers()
		case <-d.processTaskUpdatesTrigger:
			d.processTaskUpdates()
			batchTimer.Reset(maxBatchInterval)
		case <-batchTimer.C:
			d.processTaskUpdates()
			batchTimer.Reset(maxBatchInterval)
		case v := <-configWatcher:
			cluster := v.(state.EventUpdateCluster)
			d.mu.Lock()
			if cluster.Cluster.Spec.Dispatcher.HeartbeatPeriod != nil {
				// ignore error, since Spec has passed validation before
				heartbeatPeriod, _ := ptypes.Duration(cluster.Cluster.Spec.Dispatcher.HeartbeatPeriod)
				if heartbeatPeriod != d.config.HeartbeatPeriod {
					// only call d.nodes.updatePeriod when heartbeatPeriod changes
					d.config.HeartbeatPeriod = heartbeatPeriod
					d.nodes.updatePeriod(d.config.HeartbeatPeriod, d.config.HeartbeatEpsilon, d.config.GracePeriodMultiplier)
				}
			}
			d.networkBootstrapKeys = cluster.Cluster.NetworkBootstrapKeys
			d.mu.Unlock()
			d.keyMgrQueue.Publish(struct{}{})
		case <-d.ctx.Done():
			return nil
		}
	}
}

// Stop stops dispatcher and closes all grpc streams.
func (d *Dispatcher) Stop() error {
	d.mu.Lock()
	if !d.isRunning() {
		d.mu.Unlock()
		return fmt.Errorf("dispatcher is already stopped")
	}
	d.cancel()
	d.mu.Unlock()
	d.nodes.Clean()
	return nil
}

func (d *Dispatcher) isRunningLocked() error {
	d.mu.Lock()
	if !d.isRunning() {
		d.mu.Unlock()
		return grpc.Errorf(codes.Aborted, "dispatcher is stopped")
	}
	d.mu.Unlock()
	return nil
}

func (d *Dispatcher) markNodesUnknown(ctx context.Context) error {
	log := log.G(ctx).WithField("method", "(*Dispatcher).markNodesUnknown")
	var nodes []*api.Node
	var err error
	d.store.View(func(tx store.ReadTx) {
		nodes, err = store.FindNodes(tx, store.All)
	})
	if err != nil {
		return fmt.Errorf("failed to get list of nodes: %v", err)
	}
	_, err = d.store.Batch(func(batch *store.Batch) error {
		for _, n := range nodes {
			err := batch.Update(func(tx store.Tx) error {
				// check if node is still here
				node := store.GetNode(tx, n.ID)
				if node == nil {
					return nil
				}
				// do not try to resurrect down nodes
				if node.Status.State == api.NodeStatus_DOWN {
					return nil
				}
				node.Status = api.NodeStatus{
					State:   api.NodeStatus_UNKNOWN,
					Message: `Node moved to "unknown" state due to leadership change in cluster`,
				}
				nodeID := node.ID

				expireFunc := func() {
					log := log.WithField("node", nodeID)
					nodeStatus := api.NodeStatus{State: api.NodeStatus_DOWN, Message: `heartbeat failure for node in "unknown" state`}
					log.Debugf("heartbeat expiration for unknown node")
					if err := d.nodeRemove(nodeID, nodeStatus); err != nil {
						log.WithError(err).Errorf(`failed deregistering node after heartbeat expiration for node in "unknown" state`)
					}
				}
				if err := d.nodes.AddUnknown(node, expireFunc); err != nil {
					return fmt.Errorf(`adding node in "unknown" state to node store failed: %v`, err)
				}
				if err := store.UpdateNode(tx, node); err != nil {
					return fmt.Errorf("update failed %v", err)
				}
				return nil
			})
			if err != nil {
				log.WithField("node", n.ID).WithError(err).Errorf(`failed to move node to "unknown" state`)
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

// register is used for registration of node with particular dispatcher.
func (d *Dispatcher) register(ctx context.Context, nodeID string, description *api.NodeDescription) (string, error) {
	// prevent register until we're ready to accept it
	if err := d.isRunningLocked(); err != nil {
		return "", err
	}

	if err := d.nodes.CheckRateLimit(nodeID); err != nil {
		return "", err
	}

	// create or update node in store
	// TODO(stevvooe): Validate node specification.
	var node *api.Node
	err := d.store.Update(func(tx store.Tx) error {
		node = store.GetNode(tx, nodeID)
		if node == nil {
			return ErrNodeNotFound
		}

		node.Description = description
		node.Status = api.NodeStatus{
			State: api.NodeStatus_READY,
		}
		return store.UpdateNode(tx, node)

	})
	if err != nil {
		return "", err
	}

	expireFunc := func() {
		nodeStatus := api.NodeStatus{State: api.NodeStatus_DOWN, Message: "heartbeat failure"}
		log.G(ctx).Debugf("heartbeat expiration")
		if err := d.nodeRemove(nodeID, nodeStatus); err != nil {
			log.G(ctx).WithError(err).Errorf("failed deregistering node after heartbeat expiration")
		}
	}

	rn := d.nodes.Add(node, expireFunc)

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

	if err := d.isRunningLocked(); err != nil {
		return nil, err
	}

	if _, err := d.nodes.GetWithSession(nodeID, r.SessionID); err != nil {
		return nil, err
	}

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
			log.WithField("task.id", u.TaskID).Warn("cannot find target task in store")
			continue
		}

		if t.NodeID != nodeID {
			err := grpc.Errorf(codes.PermissionDenied, "cannot update a task not assigned this node")
			log.WithField("task.id", u.TaskID).Error(err)
			return nil, err
		}
	}

	d.taskUpdatesLock.Lock()
	// Enqueue task updates
	for _, u := range r.Updates {
		if u.Status == nil {
			continue
		}
		d.taskUpdates[u.TaskID] = u.Status
	}

	numUpdates := len(d.taskUpdates)
	d.taskUpdatesLock.Unlock()

	if numUpdates >= maxBatchItems {
		d.processTaskUpdatesTrigger <- struct{}{}
	}
	return nil, nil
}

func (d *Dispatcher) processTaskUpdates() {
	d.taskUpdatesLock.Lock()
	if len(d.taskUpdates) == 0 {
		d.taskUpdatesLock.Unlock()
		return
	}
	taskUpdates := d.taskUpdates
	d.taskUpdates = make(map[string]*api.TaskStatus)
	d.taskUpdatesLock.Unlock()

	log := log.G(d.ctx).WithFields(logrus.Fields{
		"method": "(*Dispatcher).processTaskUpdates",
	})

	_, err := d.store.Batch(func(batch *store.Batch) error {
		for taskID, status := range taskUpdates {
			err := batch.Update(func(tx store.Tx) error {
				logger := log.WithField("task.id", taskID)
				task := store.GetTask(tx, taskID)
				if task == nil {
					logger.Errorf("task unavailable")
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

				task.Status = *status
				if err := store.UpdateTask(tx, task); err != nil {
					logger.WithError(err).Error("failed to update task status")
					return nil
				}
				logger.Debug("task status updated")
				return nil
			})
			if err != nil {
				log.WithError(err).Error("dispatcher transaction failed")
			}
		}
		return nil
	})
	if err != nil {
		log.WithError(err).Error("dispatcher batch failed")
	}
}

// Tasks is a stream of tasks state for node. Each message contains full list
// of tasks which should be run on node, if task is not present in that list,
// it should be terminated.
func (d *Dispatcher) Tasks(r *api.TasksRequest, stream api.Dispatcher_TasksServer) error {
	nodeInfo, err := ca.RemoteNode(stream.Context())
	if err != nil {
		return err
	}
	nodeID := nodeInfo.NodeID

	if err := d.isRunningLocked(); err != nil {
		return err
	}

	fields := logrus.Fields{
		"node.id":      nodeID,
		"node.session": r.SessionID,
		"method":       "(*Dispatcher).Tasks",
	}
	if nodeInfo.ForwardedBy != nil {
		fields["forwarder.id"] = nodeInfo.ForwardedBy.NodeID
	}
	log.G(stream.Context()).WithFields(fields).Debugf("")

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
		state.EventCreateTask{Task: &api.Task{NodeID: nodeID},
			Checks: []state.TaskCheckFunc{state.TaskCheckNodeID}},
		state.EventUpdateTask{Task: &api.Task{NodeID: nodeID},
			Checks: []state.TaskCheckFunc{state.TaskCheckNodeID}},
		state.EventDeleteTask{Task: &api.Task{NodeID: nodeID},
			Checks: []state.TaskCheckFunc{state.TaskCheckNodeID}},
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
		const modificationBatchLimit = 200
		const eventPausedGap = 50 * time.Millisecond
		var modificationCnt int
		// eventPaused is true when there have been modifications
		// but next event has not arrived within eventPausedGap
		eventPaused := false

		for modificationCnt < modificationBatchLimit && !eventPaused {
			select {
			case event := <-nodeTasks:
				switch v := event.(type) {
				case state.EventCreateTask:
					tasksMap[v.Task.ID] = v.Task
					modificationCnt++
				case state.EventUpdateTask:
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
				case state.EventDeleteTask:
					delete(tasksMap, v.Task.ID)
					modificationCnt++
				}
			case <-time.After(eventPausedGap):
				if modificationCnt > 0 {
					eventPaused = true
				}
			case <-stream.Context().Done():
				return stream.Context().Err()
			case <-d.ctx.Done():
				return d.ctx.Err()
			}
		}
	}
}

func (d *Dispatcher) nodeRemove(id string, status api.NodeStatus) error {
	if err := d.isRunningLocked(); err != nil {
		return err
	}
	// TODO(aaronl): Is it worth batching node removals?
	err := d.store.Update(func(tx store.Tx) error {
		node := store.GetNode(tx, id)
		if node == nil {
			return errors.New("node not found")
		}
		node.Status = status
		return store.UpdateNode(tx, node)
	})
	if err != nil {
		return fmt.Errorf("failed to update node %s status to down: %v", id, err)
	}

	if rn := d.nodes.Delete(id); rn == nil {
		return fmt.Errorf("node %s is not found in local storage", id)
	}

	return nil
}

// Heartbeat is heartbeat method for nodes. It returns new TTL in response.
// Node should send new heartbeat earlier than now + TTL, otherwise it will
// be deregistered from dispatcher and its status will be updated to NodeStatus_DOWN
func (d *Dispatcher) Heartbeat(ctx context.Context, r *api.HeartbeatRequest) (*api.HeartbeatResponse, error) {
	nodeInfo, err := ca.RemoteNode(ctx)
	if err != nil {
		return nil, err
	}

	period, err := d.nodes.Heartbeat(nodeInfo.NodeID, r.SessionID)
	return &api.HeartbeatResponse{Period: *ptypes.DurationProto(period)}, err
}

func (d *Dispatcher) getManagers() []*api.WeightedPeer {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastSeenManagers
}

// Session is a stream which controls agent connection.
// Each message contains list of backup Managers with weights. Also there is
// a special boolean field Disconnect which if true indicates that node should
// reconnect to another Manager immediately.
func (d *Dispatcher) Session(r *api.SessionRequest, stream api.Dispatcher_SessionServer) error {
	ctx := stream.Context()
	nodeInfo, err := ca.RemoteNode(ctx)
	if err != nil {
		return err
	}
	nodeID := nodeInfo.NodeID

	if err := d.isRunningLocked(); err != nil {
		return err
	}

	// register the node.
	sessionID, err := d.register(stream.Context(), nodeID, r.Description)
	if err != nil {
		return err
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
	}, state.EventUpdateNode{Node: &api.Node{ID: nodeID},
		Checks: []state.NodeCheckFunc{state.NodeCheckID}},
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

	if err := stream.Send(&api.SessionMessage{
		SessionID:            sessionID,
		Node:                 nodeObj,
		Managers:             d.getManagers(),
		NetworkBootstrapKeys: d.networkBootstrapKeys,
	}); err != nil {
		return err
	}

	managerUpdates, mgrCancel := d.mgrQueue.Watch()
	defer mgrCancel()
	keyMgrUpdates, keyMgrCancel := d.keyMgrQueue.Watch()
	defer keyMgrCancel()

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

		nodeStatus := api.NodeStatus{State: api.NodeStatus_DISCONNECTED, Message: "node is currently trying to find new manager"}
		if err := d.nodeRemove(nodeID, nodeStatus); err != nil {
			log.WithError(err).Error("failed to remove node")
		}
		// still return an abort if the transport closure was ineffective.
		return grpc.Errorf(codes.Aborted, "node must disconnect")
	}

	for {
		// After each message send, we need to check the nodes sessionID hasn't
		// changed. If it has, we will the stream and make the node
		// re-register.
		node, err := d.nodes.GetWithSession(nodeID, sessionID)
		if err != nil {
			return err
		}

		var mgrs []*api.WeightedPeer

		var disconnect bool

		select {
		case ev := <-managerUpdates:
			mgrs = ev.([]*api.WeightedPeer)
		case ev := <-nodeUpdates:
			nodeObj = ev.(state.EventUpdateNode).Node
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-node.Disconnect:
			disconnect = true
		case <-d.ctx.Done():
			disconnect = true
		case <-keyMgrUpdates:
		}
		if mgrs == nil {
			mgrs = d.getManagers()
		}

		if err := stream.Send(&api.SessionMessage{
			SessionID:            sessionID,
			Node:                 nodeObj,
			Managers:             mgrs,
			NetworkBootstrapKeys: d.networkBootstrapKeys,
		}); err != nil {
			return err
		}
		if disconnect {
			return disconnectNode()
		}
	}
}

// NodeCount returns number of nodes which connected to this dispatcher.
func (d *Dispatcher) NodeCount() int {
	return d.nodes.Len()
}
