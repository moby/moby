package dispatcher

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/transport"

	"github.com/Sirupsen/logrus"
	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/equality"
	"github.com/docker/swarmkit/ca"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/manager/state/watch"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"github.com/docker/swarmkit/remotes"
	"github.com/pkg/errors"
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

	modificationBatchLimit = 100
	batchingWaitTime       = 100 * time.Millisecond
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

// Dispatcher is responsible for dispatching tasks and tracking agent health.
type Dispatcher struct {
	mu                   sync.Mutex
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

	nodeUpdates     map[string]nodeUpdate // indexed by node ID
	nodeUpdatesLock sync.Mutex

	processUpdatesTrigger chan struct{}

	// for waiting for the next task/node batch update
	processUpdatesLock sync.Mutex
	processUpdatesCond *sync.Cond
}

// New returns Dispatcher with cluster interface(usually raft.Node).
// NOTE: each handler which does something with raft must add to Dispatcher.wg
func New(cluster Cluster, c *Config) *Dispatcher {
	d := &Dispatcher{
		nodes:                 newNodeStore(c.HeartbeatPeriod, c.HeartbeatEpsilon, c.GracePeriodMultiplier, c.RateLimitPeriod),
		store:                 cluster.MemoryStore(),
		cluster:               cluster,
		taskUpdates:           make(map[string]*api.TaskStatus),
		nodeUpdates:           make(map[string]nodeUpdate),
		processUpdatesTrigger: make(chan struct{}, 1),
		config:                c,
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
	d.mu.Lock()
	if d.isRunning() {
		d.mu.Unlock()
		return errors.New("dispatcher is already running")
	}
	ctx = log.WithModule(ctx, "dispatcher")
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
	// set queues here to guarantee that Close will close them
	d.mgrQueue = watch.NewQueue()
	d.keyMgrQueue = watch.NewQueue()

	peerWatcher, peerCancel := d.cluster.SubscribePeers()
	defer peerCancel()
	d.lastSeenManagers = getWeightedPeers(d.cluster)

	defer cancel()
	d.ctx, d.cancel = context.WithCancel(ctx)
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
		d.mgrQueue.Publish(mgrs)
	}

	batchTimer := time.NewTimer(maxBatchInterval)
	defer batchTimer.Stop()

	for {
		select {
		case ev := <-peerWatcher:
			publishManagers(ev.([]*api.Peer))
		case <-d.processUpdatesTrigger:
			d.processUpdates()
			batchTimer.Reset(maxBatchInterval)
		case <-batchTimer.C:
			d.processUpdates()
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
			d.keyMgrQueue.Publish(cluster.Cluster.NetworkBootstrapKeys)
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
		return errors.New("dispatcher is already stopped")
	}
	d.cancel()
	d.mu.Unlock()
	d.nodes.Clean()

	d.processUpdatesLock.Lock()
	// In case there are any waiters. There is no chance of any starting
	// after this point, because they check if the context is canceled
	// before waiting.
	d.processUpdatesCond.Broadcast()
	d.processUpdatesLock.Unlock()

	d.mgrQueue.Close()
	d.keyMgrQueue.Close()

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
		return errors.Wrap(err, "failed to get list of nodes")
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
					return errors.Wrap(err, `adding node in "unknown" state to node store failed`)
				}
				if err := store.UpdateNode(tx, node); err != nil {
					return errors.Wrap(err, "update failed")
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

// updateNode updates the description of a node and sets status to READY
// this is used during registration when a new node description is provided
// and during node updates when the node description changes
func (d *Dispatcher) updateNode(nodeID string, description *api.NodeDescription) error {
	d.nodeUpdatesLock.Lock()
	d.nodeUpdates[nodeID] = nodeUpdate{status: &api.NodeStatus{State: api.NodeStatus_READY}, description: description}
	numUpdates := len(d.nodeUpdates)
	d.nodeUpdatesLock.Unlock()

	if numUpdates >= maxBatchItems {
		select {
		case d.processUpdatesTrigger <- struct{}{}:
		case <-d.ctx.Done():
			return d.ctx.Err()
		}

	}

	// Wait until the node update batch happens before unblocking register.
	d.processUpdatesLock.Lock()
	select {
	case <-d.ctx.Done():
		return d.ctx.Err()
	default:
	}
	d.processUpdatesCond.Wait()
	d.processUpdatesLock.Unlock()

	return nil
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

	// TODO(stevvooe): Validate node specification.
	var node *api.Node
	d.store.View(func(tx store.ReadTx) {
		node = store.GetNode(tx, nodeID)
	})
	if node == nil {
		return "", ErrNodeNotFound
	}

	if err := d.updateNode(nodeID, description); err != nil {
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
		select {
		case d.processUpdatesTrigger <- struct{}{}:
		case <-d.ctx.Done():
		}
	}
	return nil, nil
}

func (d *Dispatcher) processUpdates() {
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

	log := log.G(d.ctx).WithFields(logrus.Fields{
		"method": "(*Dispatcher).processUpdates",
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
					node.Status = *nodeUpdate.status
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
			case <-d.ctx.Done():
				return d.ctx.Err()
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
		"method":       "(*Dispatcher).Assignments",
	}
	if nodeInfo.ForwardedBy != nil {
		fields["forwarder.id"] = nodeInfo.ForwardedBy.NodeID
	}
	log := log.G(stream.Context()).WithFields(fields)
	log.Debugf("")

	if _, err = d.nodes.GetWithSession(nodeID, r.SessionID); err != nil {
		return err
	}

	var (
		sequence  int64
		appliesTo string
		initial   api.AssignmentsMessage
	)
	tasksMap := make(map[string]*api.Task)

	sendMessage := func(msg api.AssignmentsMessage, assignmentType api.AssignmentsMessage_Type) error {
		sequence++
		msg.AppliesTo = appliesTo
		msg.ResultsIn = strconv.FormatInt(sequence, 10)
		appliesTo = msg.ResultsIn
		msg.Type = assignmentType

		if err := stream.Send(&msg); err != nil {
			return err
		}
		return nil
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
				// We only care about tasks that are ASSIGNED or
				// higher. If the state is below ASSIGNED, the
				// task may not meet the constraints for this
				// node, so we have to be careful about sending
				// secrets associated with it.
				if t.Status.State < api.TaskStateAssigned {
					continue
				}

				tasksMap[t.ID] = t
				initial.UpdateTasks = append(initial.UpdateTasks, t)
			}
			return nil
		},
		state.EventUpdateTask{Task: &api.Task{NodeID: nodeID},
			Checks: []state.TaskCheckFunc{state.TaskCheckNodeID}},
		state.EventDeleteTask{Task: &api.Task{NodeID: nodeID},
			Checks: []state.TaskCheckFunc{state.TaskCheckNodeID}},
	)
	if err != nil {
		return err
	}
	defer cancel()

	if err := sendMessage(initial, api.AssignmentsMessage_COMPLETE); err != nil {
		return err
	}

	for {
		// Check for session expiration
		if _, err := d.nodes.GetWithSession(nodeID, r.SessionID); err != nil {
			return err
		}

		// bursty events should be processed in batches and sent out together
		var (
			update          api.AssignmentsMessage
			modificationCnt int
			batchingTimer   *time.Timer
			batchingTimeout <-chan time.Time
			updateTasks     = make(map[string]*api.Task)
			removeTasks     = make(map[string]struct{})
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
				case state.EventUpdateTask:
					// We only care about tasks that are ASSIGNED or
					// higher.
					if v.Task.Status.State < api.TaskStateAssigned {
						continue
					}

					if oldTask, exists := tasksMap[v.Task.ID]; exists {
						// States ASSIGNED and below are set by the orchestrator/scheduler,
						// not the agent, so tasks in these states need to be sent to the
						// agent even if nothing else has changed.
						if equality.TasksEqualStable(oldTask, v.Task) && v.Task.Status.State > api.TaskStateAssigned {
							// this update should not trigger a task change for the agent
							tasksMap[v.Task.ID] = v.Task
							continue
						}
					}
					tasksMap[v.Task.ID] = v.Task
					updateTasks[v.Task.ID] = v.Task

					oneModification()
				case state.EventDeleteTask:

					if _, exists := tasksMap[v.Task.ID]; !exists {
						continue
					}

					removeTasks[v.Task.ID] = struct{}{}

					delete(tasksMap, v.Task.ID)

					oneModification()
				}
			case <-batchingTimeout:
				break batchingLoop
			case <-stream.Context().Done():
				return stream.Context().Err()
			case <-d.ctx.Done():
				return d.ctx.Err()
			}
		}

		if batchingTimer != nil {
			batchingTimer.Stop()
		}

		if modificationCnt > 0 {
			for id, task := range updateTasks {
				if _, ok := removeTasks[id]; !ok {
					update.UpdateTasks = append(update.UpdateTasks, task)
				}
			}
			for id := range removeTasks {
				update.RemoveTasks = append(update.RemoveTasks, id)
			}
			if err := sendMessage(update, api.AssignmentsMessage_INCREMENTAL); err != nil {
				return err
			}
		}
	}
}

func (d *Dispatcher) nodeRemove(id string, status api.NodeStatus) error {
	if err := d.isRunningLocked(); err != nil {
		return err
	}

	d.nodeUpdatesLock.Lock()
	d.nodeUpdates[id] = nodeUpdate{status: status.Copy(), description: d.nodeUpdates[id].description}
	numUpdates := len(d.nodeUpdates)
	d.nodeUpdatesLock.Unlock()

	if numUpdates >= maxBatchItems {
		select {
		case d.processUpdatesTrigger <- struct{}{}:
		case <-d.ctx.Done():
		}
	}

	if rn := d.nodes.Delete(id); rn == nil {
		return errors.Errorf("node %s is not found in local storage", id)
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

func (d *Dispatcher) getNetworkBootstrapKeys() []*api.EncryptionKey {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.networkBootstrapKeys
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

	var sessionID string
	if _, err := d.nodes.GetWithSession(nodeID, r.SessionID); err != nil {
		// register the node.
		sessionID, err = d.register(stream.Context(), nodeID, r.Description)
		if err != nil {
			return err
		}
	} else {
		sessionID = r.SessionID
		// update the node description
		if err := d.updateNode(nodeID, r.Description); err != nil {
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
		NetworkBootstrapKeys: d.getNetworkBootstrapKeys(),
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
		)

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
		case ev := <-keyMgrUpdates:
			netKeys = ev.([]*api.EncryptionKey)
		}
		if mgrs == nil {
			mgrs = d.getManagers()
		}
		if netKeys == nil {
			netKeys = d.getNetworkBootstrapKeys()
		}

		if err := stream.Send(&api.SessionMessage{
			SessionID:            sessionID,
			Node:                 nodeObj,
			Managers:             mgrs,
			NetworkBootstrapKeys: netKeys,
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
