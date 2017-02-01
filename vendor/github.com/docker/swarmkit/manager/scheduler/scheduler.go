package scheduler

import (
	"container/list"
	"time"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"golang.org/x/net/context"
)

const (
	// monitorFailures is the lookback period for counting failures of
	// a task to determine if a node is faulty for a particular service.
	monitorFailures = 5 * time.Minute

	// maxFailures is the number of failures within monitorFailures that
	// triggers downweighting of a node in the sorting function.
	maxFailures = 5
)

type schedulingDecision struct {
	old *api.Task
	new *api.Task
}

// Scheduler assigns tasks to nodes.
type Scheduler struct {
	store           *store.MemoryStore
	unassignedTasks *list.List
	// preassignedTasks already have NodeID, need resource validation
	preassignedTasks map[string]*api.Task
	nodeSet          nodeSet
	allTasks         map[string]*api.Task
	pipeline         *Pipeline

	// stopChan signals to the state machine to stop running
	stopChan chan struct{}
	// doneChan is closed when the state machine terminates
	doneChan chan struct{}
}

// New creates a new scheduler.
func New(store *store.MemoryStore) *Scheduler {
	return &Scheduler{
		store:            store,
		unassignedTasks:  list.New(),
		preassignedTasks: make(map[string]*api.Task),
		allTasks:         make(map[string]*api.Task),
		stopChan:         make(chan struct{}),
		doneChan:         make(chan struct{}),
		pipeline:         NewPipeline(),
	}
}

func (s *Scheduler) setupTasksList(tx store.ReadTx) error {
	tasks, err := store.FindTasks(tx, store.All)
	if err != nil {
		return err
	}

	tasksByNode := make(map[string]map[string]*api.Task)
	for _, t := range tasks {
		// Ignore all tasks that have not reached PENDING
		// state and tasks that no longer consume resources.
		if t.Status.State < api.TaskStatePending || t.Status.State > api.TaskStateRunning {
			continue
		}

		s.allTasks[t.ID] = t
		if t.NodeID == "" {
			s.enqueue(t)
			continue
		}
		// preassigned tasks need to validate resource requirement on corresponding node
		if t.Status.State == api.TaskStatePending {
			s.preassignedTasks[t.ID] = t
			continue
		}

		if tasksByNode[t.NodeID] == nil {
			tasksByNode[t.NodeID] = make(map[string]*api.Task)
		}
		tasksByNode[t.NodeID][t.ID] = t
	}

	if err := s.buildNodeSet(tx, tasksByNode); err != nil {
		return err
	}

	return nil
}

// Run is the scheduler event loop.
func (s *Scheduler) Run(ctx context.Context) error {
	defer close(s.doneChan)

	updates, cancel, err := store.ViewAndWatch(s.store, s.setupTasksList)
	if err != nil {
		log.G(ctx).WithError(err).Errorf("snapshot store update failed")
		return err
	}
	defer cancel()

	// Validate resource for tasks from preassigned tasks
	// do this before other tasks because preassigned tasks like
	// global service should start before other tasks
	s.processPreassignedTasks(ctx)

	// Queue all unassigned tasks before processing changes.
	s.tick(ctx)

	const (
		// commitDebounceGap is the amount of time to wait between
		// commit events to debounce them.
		commitDebounceGap = 50 * time.Millisecond
		// maxLatency is a time limit on the debouncing.
		maxLatency = time.Second
	)
	var (
		debouncingStarted     time.Time
		commitDebounceTimer   *time.Timer
		commitDebounceTimeout <-chan time.Time
	)

	pendingChanges := 0

	schedule := func() {
		if len(s.preassignedTasks) > 0 {
			s.processPreassignedTasks(ctx)
		}
		if pendingChanges > 0 {
			s.tick(ctx)
			pendingChanges = 0
		}
	}

	// Watch for changes.
	for {
		select {
		case event := <-updates:
			switch v := event.(type) {
			case state.EventCreateTask:
				pendingChanges += s.createTask(ctx, v.Task)
			case state.EventUpdateTask:
				pendingChanges += s.updateTask(ctx, v.Task)
			case state.EventDeleteTask:
				s.deleteTask(ctx, v.Task)
			case state.EventCreateNode:
				s.createOrUpdateNode(v.Node)
				pendingChanges++
			case state.EventUpdateNode:
				s.createOrUpdateNode(v.Node)
				pendingChanges++
			case state.EventDeleteNode:
				s.nodeSet.remove(v.Node.ID)
			case state.EventCommit:
				if commitDebounceTimer != nil {
					if time.Since(debouncingStarted) > maxLatency {
						commitDebounceTimer.Stop()
						commitDebounceTimer = nil
						commitDebounceTimeout = nil
						schedule()
					} else {
						commitDebounceTimer.Reset(commitDebounceGap)
					}
				} else {
					commitDebounceTimer = time.NewTimer(commitDebounceGap)
					commitDebounceTimeout = commitDebounceTimer.C
					debouncingStarted = time.Now()
				}
			}
		case <-commitDebounceTimeout:
			schedule()
			commitDebounceTimer = nil
			commitDebounceTimeout = nil
		case <-s.stopChan:
			return nil
		}
	}
}

// Stop causes the scheduler event loop to stop running.
func (s *Scheduler) Stop() {
	close(s.stopChan)
	<-s.doneChan
}

// enqueue queues a task for scheduling.
func (s *Scheduler) enqueue(t *api.Task) {
	s.unassignedTasks.PushBack(t)
}

func (s *Scheduler) createTask(ctx context.Context, t *api.Task) int {
	// Ignore all tasks that have not reached PENDING
	// state, and tasks that no longer consume resources.
	if t.Status.State < api.TaskStatePending || t.Status.State > api.TaskStateRunning {
		return 0
	}

	s.allTasks[t.ID] = t
	if t.NodeID == "" {
		// unassigned task
		s.enqueue(t)
		return 1
	}

	if t.Status.State == api.TaskStatePending {
		s.preassignedTasks[t.ID] = t
		// preassigned tasks do not contribute to running tasks count
		return 0
	}

	nodeInfo, err := s.nodeSet.nodeInfo(t.NodeID)
	if err == nil && nodeInfo.addTask(t) {
		s.nodeSet.updateNode(nodeInfo)
	}

	return 0
}

func (s *Scheduler) updateTask(ctx context.Context, t *api.Task) int {
	// Ignore all tasks that have not reached PENDING
	// state.
	if t.Status.State < api.TaskStatePending {
		return 0
	}

	oldTask := s.allTasks[t.ID]

	// Ignore all tasks that have not reached ALLOCATED
	// state, and tasks that no longer consume resources.
	if t.Status.State > api.TaskStateRunning {
		if oldTask == nil {
			return 1
		}
		s.deleteTask(ctx, oldTask)
		if t.Status.State != oldTask.Status.State &&
			(t.Status.State == api.TaskStateFailed || t.Status.State == api.TaskStateRejected) {
			nodeInfo, err := s.nodeSet.nodeInfo(t.NodeID)
			if err == nil {
				nodeInfo.taskFailed(ctx, t.ServiceID)
				s.nodeSet.updateNode(nodeInfo)
			}
		}
		return 1
	}

	if t.NodeID == "" {
		// unassigned task
		if oldTask != nil {
			s.deleteTask(ctx, oldTask)
		}
		s.allTasks[t.ID] = t
		s.enqueue(t)
		return 1
	}

	if t.Status.State == api.TaskStatePending {
		if oldTask != nil {
			s.deleteTask(ctx, oldTask)
		}
		s.allTasks[t.ID] = t
		s.preassignedTasks[t.ID] = t
		// preassigned tasks do not contribute to running tasks count
		return 0
	}

	s.allTasks[t.ID] = t
	nodeInfo, err := s.nodeSet.nodeInfo(t.NodeID)
	if err == nil && nodeInfo.addTask(t) {
		s.nodeSet.updateNode(nodeInfo)
	}

	return 0
}

func (s *Scheduler) deleteTask(ctx context.Context, t *api.Task) {
	delete(s.allTasks, t.ID)
	delete(s.preassignedTasks, t.ID)
	nodeInfo, err := s.nodeSet.nodeInfo(t.NodeID)
	if err == nil && nodeInfo.removeTask(t) {
		s.nodeSet.updateNode(nodeInfo)
	}
}

func (s *Scheduler) createOrUpdateNode(n *api.Node) {
	nodeInfo, _ := s.nodeSet.nodeInfo(n.ID)
	var resources api.Resources
	if n.Description != nil && n.Description.Resources != nil {
		resources = *n.Description.Resources
		// reconcile resources by looping over all tasks in this node
		for _, task := range nodeInfo.Tasks {
			reservations := taskReservations(task.Spec)
			resources.MemoryBytes -= reservations.MemoryBytes
			resources.NanoCPUs -= reservations.NanoCPUs
		}
	}
	nodeInfo.Node = n
	nodeInfo.AvailableResources = resources
	s.nodeSet.addOrUpdateNode(nodeInfo)
}

func (s *Scheduler) processPreassignedTasks(ctx context.Context) {
	schedulingDecisions := make(map[string]schedulingDecision, len(s.preassignedTasks))
	for _, t := range s.preassignedTasks {
		newT := s.taskFitNode(ctx, t, t.NodeID)
		if newT == nil {
			continue
		}
		schedulingDecisions[t.ID] = schedulingDecision{old: t, new: newT}
	}

	successful, failed := s.applySchedulingDecisions(ctx, schedulingDecisions)

	for _, decision := range successful {
		if decision.new.Status.State == api.TaskStateAssigned {
			delete(s.preassignedTasks, decision.old.ID)
		}
	}
	for _, decision := range failed {
		s.allTasks[decision.old.ID] = decision.old
		nodeInfo, err := s.nodeSet.nodeInfo(decision.new.NodeID)
		if err == nil && nodeInfo.removeTask(decision.new) {
			s.nodeSet.updateNode(nodeInfo)
		}
	}
}

// tick attempts to schedule the queue.
func (s *Scheduler) tick(ctx context.Context) {
	tasksByCommonSpec := make(map[string]map[string]*api.Task)
	schedulingDecisions := make(map[string]schedulingDecision, s.unassignedTasks.Len())

	var next *list.Element
	for e := s.unassignedTasks.Front(); e != nil; e = next {
		next = e.Next()
		t := s.allTasks[e.Value.(*api.Task).ID]
		if t == nil || t.NodeID != "" {
			// task deleted or already assigned
			s.unassignedTasks.Remove(e)
			continue
		}

		// Group common tasks with common specs by marshalling the spec
		// into taskKey and using it as a map key.
		// TODO(aaronl): Once specs are versioned, this will allow a
		// much more efficient fast path.
		fieldsToMarshal := api.Task{
			ServiceID: t.ServiceID,
			Spec:      t.Spec,
		}
		marshalled, err := fieldsToMarshal.Marshal()
		if err != nil {
			panic(err)
		}
		taskGroupKey := string(marshalled)

		if tasksByCommonSpec[taskGroupKey] == nil {
			tasksByCommonSpec[taskGroupKey] = make(map[string]*api.Task)
		}
		tasksByCommonSpec[taskGroupKey][t.ID] = t
		s.unassignedTasks.Remove(e)
	}

	for _, taskGroup := range tasksByCommonSpec {
		s.scheduleTaskGroup(ctx, taskGroup, schedulingDecisions)
	}

	_, failed := s.applySchedulingDecisions(ctx, schedulingDecisions)
	for _, decision := range failed {
		s.allTasks[decision.old.ID] = decision.old

		nodeInfo, err := s.nodeSet.nodeInfo(decision.new.NodeID)
		if err == nil && nodeInfo.removeTask(decision.new) {
			s.nodeSet.updateNode(nodeInfo)
		}

		// enqueue task for next scheduling attempt
		s.enqueue(decision.old)
	}
}

func (s *Scheduler) applySchedulingDecisions(ctx context.Context, schedulingDecisions map[string]schedulingDecision) (successful, failed []schedulingDecision) {
	if len(schedulingDecisions) == 0 {
		return
	}

	successful = make([]schedulingDecision, 0, len(schedulingDecisions))

	// Apply changes to master store
	applied, err := s.store.Batch(func(batch *store.Batch) error {
		for len(schedulingDecisions) > 0 {
			err := batch.Update(func(tx store.Tx) error {
				// Update exactly one task inside this Update
				// callback.
				for taskID, decision := range schedulingDecisions {
					delete(schedulingDecisions, taskID)

					t := store.GetTask(tx, taskID)
					if t == nil {
						// Task no longer exists
						nodeInfo, err := s.nodeSet.nodeInfo(decision.new.NodeID)
						if err == nil && nodeInfo.removeTask(decision.new) {
							s.nodeSet.updateNode(nodeInfo)
						}
						delete(s.allTasks, decision.old.ID)

						continue
					}

					if t.Status.State == decision.new.Status.State && t.Status.Message == decision.new.Status.Message {
						// No changes, ignore
						continue
					}

					if t.Status.State >= api.TaskStateAssigned {
						nodeInfo, err := s.nodeSet.nodeInfo(decision.new.NodeID)
						if err != nil {
							failed = append(failed, decision)
							continue
						}
						node := store.GetNode(tx, decision.new.NodeID)
						if node == nil || node.Meta.Version != nodeInfo.Meta.Version {
							// node is out of date
							failed = append(failed, decision)
							continue
						}
					}

					if err := store.UpdateTask(tx, decision.new); err != nil {
						log.G(ctx).Debugf("scheduler failed to update task %s; will retry", taskID)
						failed = append(failed, decision)
						continue
					}
					successful = append(successful, decision)
					return nil
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		log.G(ctx).WithError(err).Error("scheduler tick transaction failed")
		failed = append(failed, successful[applied:]...)
		successful = successful[:applied]
	}
	return
}

// taskFitNode checks if a node has enough resources to accommodate a task.
func (s *Scheduler) taskFitNode(ctx context.Context, t *api.Task, nodeID string) *api.Task {
	nodeInfo, err := s.nodeSet.nodeInfo(nodeID)
	if err != nil {
		// node does not exist in set (it may have been deleted)
		return nil
	}
	newT := *t
	s.pipeline.SetTask(t)
	if !s.pipeline.Process(&nodeInfo) {
		// this node cannot accommodate this task
		newT.Status.Timestamp = ptypes.MustTimestampProto(time.Now())
		newT.Status.Message = s.pipeline.Explain()
		s.allTasks[t.ID] = &newT

		return &newT
	}
	newT.Status = api.TaskStatus{
		State:     api.TaskStateAssigned,
		Timestamp: ptypes.MustTimestampProto(time.Now()),
		Message:   "scheduler confirmed task can run on preassigned node",
	}
	s.allTasks[t.ID] = &newT

	if nodeInfo.addTask(&newT) {
		s.nodeSet.updateNode(nodeInfo)
	}
	return &newT
}

// scheduleTaskGroup schedules a batch of tasks that are part of the same
// service and share the same version of the spec.
func (s *Scheduler) scheduleTaskGroup(ctx context.Context, taskGroup map[string]*api.Task, schedulingDecisions map[string]schedulingDecision) {
	// Pick at task at random from taskGroup to use for constraint
	// evaluation. It doesn't matter which one we pick because all the
	// tasks in the group are equal in terms of the fields the constraint
	// filters consider.
	var t *api.Task
	for _, t = range taskGroup {
		break
	}

	s.pipeline.SetTask(t)

	now := time.Now()

	nodeLess := func(a *NodeInfo, b *NodeInfo) bool {
		// If either node has at least maxFailures recent failures,
		// that's the deciding factor.
		recentFailuresA := a.countRecentFailures(now, t.ServiceID)
		recentFailuresB := b.countRecentFailures(now, t.ServiceID)

		if recentFailuresA >= maxFailures || recentFailuresB >= maxFailures {
			if recentFailuresA > recentFailuresB {
				return false
			}
			if recentFailuresB > recentFailuresA {
				return true
			}
		}

		tasksByServiceA := a.DesiredRunningTasksCountByService[t.ServiceID]
		tasksByServiceB := b.DesiredRunningTasksCountByService[t.ServiceID]

		if tasksByServiceA < tasksByServiceB {
			return true
		}
		if tasksByServiceA > tasksByServiceB {
			return false
		}

		// Total number of tasks breaks ties.
		return a.DesiredRunningTasksCount < b.DesiredRunningTasksCount
	}

	nodes := s.nodeSet.findBestNodes(len(taskGroup), s.pipeline.Process, nodeLess)
	nodeCount := len(nodes)
	if nodeCount == 0 {
		s.noSuitableNode(ctx, taskGroup, schedulingDecisions)
		return
	}

	failedConstraints := make(map[int]bool) // key is index in nodes slice
	nodeIter := 0
	for taskID, t := range taskGroup {
		n := &nodes[nodeIter%nodeCount]

		log.G(ctx).WithField("task.id", t.ID).Debugf("assigning to node %s", n.ID)
		newT := *t
		newT.NodeID = n.ID
		newT.Status = api.TaskStatus{
			State:     api.TaskStateAssigned,
			Timestamp: ptypes.MustTimestampProto(time.Now()),
			Message:   "scheduler assigned task to node",
		}
		s.allTasks[t.ID] = &newT

		nodeInfo, err := s.nodeSet.nodeInfo(n.ID)
		if err == nil && nodeInfo.addTask(&newT) {
			s.nodeSet.updateNode(nodeInfo)
			nodes[nodeIter%nodeCount] = nodeInfo
		}

		schedulingDecisions[taskID] = schedulingDecision{old: t, new: &newT}
		delete(taskGroup, taskID)

		if nodeIter+1 < nodeCount {
			// First pass fills the nodes until they have the same
			// number of tasks from this service.
			nextNode := nodes[(nodeIter+1)%nodeCount]
			if nodeLess(&nextNode, &nodeInfo) {
				nodeIter++
			}
		} else {
			// In later passes, we just assign one task at a time
			// to each node that still meets the constraints.
			nodeIter++
		}

		origNodeIter := nodeIter
		for failedConstraints[nodeIter%nodeCount] || !s.pipeline.Process(&nodes[nodeIter%nodeCount]) {
			failedConstraints[nodeIter%nodeCount] = true
			nodeIter++
			if nodeIter-origNodeIter == nodeCount {
				// None of the nodes meet the constraints anymore.
				s.noSuitableNode(ctx, taskGroup, schedulingDecisions)
				return
			}
		}
	}
}

func (s *Scheduler) noSuitableNode(ctx context.Context, taskGroup map[string]*api.Task, schedulingDecisions map[string]schedulingDecision) {
	explanation := s.pipeline.Explain()
	for _, t := range taskGroup {
		log.G(ctx).WithField("task.id", t.ID).Debug("no suitable node available for task")

		newT := *t
		newT.Status.Timestamp = ptypes.MustTimestampProto(time.Now())
		if explanation != "" {
			newT.Status.Message = "no suitable node (" + explanation + ")"
		} else {
			newT.Status.Message = "no suitable node"
		}
		s.allTasks[t.ID] = &newT
		schedulingDecisions[t.ID] = schedulingDecision{old: t, new: &newT}

		s.enqueue(&newT)
	}
}

func (s *Scheduler) buildNodeSet(tx store.ReadTx, tasksByNode map[string]map[string]*api.Task) error {
	nodes, err := store.FindNodes(tx, store.All)
	if err != nil {
		return err
	}

	s.nodeSet.alloc(len(nodes))

	for _, n := range nodes {
		var resources api.Resources
		if n.Description != nil && n.Description.Resources != nil {
			resources = *n.Description.Resources
		}
		s.nodeSet.addOrUpdateNode(newNodeInfo(n, tasksByNode[n.ID], resources))
	}

	return nil
}
