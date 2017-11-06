package scheduler

import (
	"time"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/genericresource"
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
	unassignedTasks map[string]*api.Task
	// pendingPreassignedTasks already have NodeID, need resource validation
	pendingPreassignedTasks map[string]*api.Task
	// preassignedTasks tracks tasks that were preassigned, including those
	// past the pending state.
	preassignedTasks map[string]struct{}
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
		store:                   store,
		unassignedTasks:         make(map[string]*api.Task),
		pendingPreassignedTasks: make(map[string]*api.Task),
		preassignedTasks:        make(map[string]struct{}),
		allTasks:                make(map[string]*api.Task),
		stopChan:                make(chan struct{}),
		doneChan:                make(chan struct{}),
		pipeline:                NewPipeline(),
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
			s.preassignedTasks[t.ID] = struct{}{}
			s.pendingPreassignedTasks[t.ID] = t
			continue
		}

		if tasksByNode[t.NodeID] == nil {
			tasksByNode[t.NodeID] = make(map[string]*api.Task)
		}
		tasksByNode[t.NodeID][t.ID] = t
	}

	return s.buildNodeSet(tx, tasksByNode)
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

	tickRequired := false

	schedule := func() {
		if len(s.pendingPreassignedTasks) > 0 {
			s.processPreassignedTasks(ctx)
		}
		if tickRequired {
			s.tick(ctx)
			tickRequired = false
		}
	}

	// Watch for changes.
	for {
		select {
		case event := <-updates:
			switch v := event.(type) {
			case api.EventCreateTask:
				if s.createTask(ctx, v.Task) {
					tickRequired = true
				}
			case api.EventUpdateTask:
				if s.updateTask(ctx, v.Task) {
					tickRequired = true
				}
			case api.EventDeleteTask:
				if s.deleteTask(v.Task) {
					// deleting tasks may free up node resource, pending tasks should be re-evaluated.
					tickRequired = true
				}
			case api.EventCreateNode:
				s.createOrUpdateNode(v.Node)
				tickRequired = true
			case api.EventUpdateNode:
				s.createOrUpdateNode(v.Node)
				tickRequired = true
			case api.EventDeleteNode:
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
	s.unassignedTasks[t.ID] = t
}

func (s *Scheduler) createTask(ctx context.Context, t *api.Task) bool {
	// Ignore all tasks that have not reached PENDING
	// state, and tasks that no longer consume resources.
	if t.Status.State < api.TaskStatePending || t.Status.State > api.TaskStateRunning {
		return false
	}

	s.allTasks[t.ID] = t
	if t.NodeID == "" {
		// unassigned task
		s.enqueue(t)
		return true
	}

	if t.Status.State == api.TaskStatePending {
		s.preassignedTasks[t.ID] = struct{}{}
		s.pendingPreassignedTasks[t.ID] = t
		// preassigned tasks do not contribute to running tasks count
		return false
	}

	nodeInfo, err := s.nodeSet.nodeInfo(t.NodeID)
	if err == nil && nodeInfo.addTask(t) {
		s.nodeSet.updateNode(nodeInfo)
	}

	return false
}

func (s *Scheduler) updateTask(ctx context.Context, t *api.Task) bool {
	// Ignore all tasks that have not reached PENDING
	// state.
	if t.Status.State < api.TaskStatePending {
		return false
	}

	oldTask := s.allTasks[t.ID]

	// Ignore all tasks that have not reached Pending
	// state, and tasks that no longer consume resources.
	if t.Status.State > api.TaskStateRunning {
		if oldTask == nil {
			return false
		}

		if t.Status.State != oldTask.Status.State &&
			(t.Status.State == api.TaskStateFailed || t.Status.State == api.TaskStateRejected) {
			// Keep track of task failures, so other nodes can be preferred
			// for scheduling this service if it looks like the service is
			// failing in a loop on this node. However, skip this for
			// preassigned tasks, because the scheduler does not choose
			// which nodes those run on.
			if _, wasPreassigned := s.preassignedTasks[t.ID]; !wasPreassigned {
				nodeInfo, err := s.nodeSet.nodeInfo(t.NodeID)
				if err == nil {
					nodeInfo.taskFailed(ctx, t)
					s.nodeSet.updateNode(nodeInfo)
				}
			}
		}

		s.deleteTask(oldTask)

		return true
	}

	if t.NodeID == "" {
		// unassigned task
		if oldTask != nil {
			s.deleteTask(oldTask)
		}
		s.allTasks[t.ID] = t
		s.enqueue(t)
		return true
	}

	if t.Status.State == api.TaskStatePending {
		if oldTask != nil {
			s.deleteTask(oldTask)
		}
		s.preassignedTasks[t.ID] = struct{}{}
		s.allTasks[t.ID] = t
		s.pendingPreassignedTasks[t.ID] = t
		// preassigned tasks do not contribute to running tasks count
		return false
	}

	s.allTasks[t.ID] = t
	nodeInfo, err := s.nodeSet.nodeInfo(t.NodeID)
	if err == nil && nodeInfo.addTask(t) {
		s.nodeSet.updateNode(nodeInfo)
	}

	return false
}

func (s *Scheduler) deleteTask(t *api.Task) bool {
	delete(s.allTasks, t.ID)
	delete(s.preassignedTasks, t.ID)
	delete(s.pendingPreassignedTasks, t.ID)
	nodeInfo, err := s.nodeSet.nodeInfo(t.NodeID)
	if err == nil && nodeInfo.removeTask(t) {
		s.nodeSet.updateNode(nodeInfo)
		return true
	}
	return false
}

func (s *Scheduler) createOrUpdateNode(n *api.Node) {
	nodeInfo, nodeInfoErr := s.nodeSet.nodeInfo(n.ID)
	var resources *api.Resources
	if n.Description != nil && n.Description.Resources != nil {
		resources = n.Description.Resources.Copy()
		// reconcile resources by looping over all tasks in this node
		if nodeInfoErr == nil {
			for _, task := range nodeInfo.Tasks {
				reservations := taskReservations(task.Spec)

				resources.MemoryBytes -= reservations.MemoryBytes
				resources.NanoCPUs -= reservations.NanoCPUs

				genericresource.ConsumeNodeResources(&resources.Generic,
					task.AssignedGenericResources)
			}
		}
	} else {
		resources = &api.Resources{}
	}

	if nodeInfoErr != nil {
		nodeInfo = newNodeInfo(n, nil, *resources)
	} else {
		nodeInfo.Node = n
		nodeInfo.AvailableResources = resources
	}
	s.nodeSet.addOrUpdateNode(nodeInfo)
}

func (s *Scheduler) processPreassignedTasks(ctx context.Context) {
	schedulingDecisions := make(map[string]schedulingDecision, len(s.pendingPreassignedTasks))
	for _, t := range s.pendingPreassignedTasks {
		newT := s.taskFitNode(ctx, t, t.NodeID)
		if newT == nil {
			continue
		}
		schedulingDecisions[t.ID] = schedulingDecision{old: t, new: newT}
	}

	successful, failed := s.applySchedulingDecisions(ctx, schedulingDecisions)

	for _, decision := range successful {
		if decision.new.Status.State == api.TaskStateAssigned {
			delete(s.pendingPreassignedTasks, decision.old.ID)
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
	type commonSpecKey struct {
		serviceID   string
		specVersion api.Version
	}
	tasksByCommonSpec := make(map[commonSpecKey]map[string]*api.Task)
	var oneOffTasks []*api.Task
	schedulingDecisions := make(map[string]schedulingDecision, len(s.unassignedTasks))

	for taskID, t := range s.unassignedTasks {
		if t == nil || t.NodeID != "" {
			// task deleted or already assigned
			delete(s.unassignedTasks, taskID)
			continue
		}

		// Group tasks with common specs
		if t.SpecVersion != nil {
			taskGroupKey := commonSpecKey{
				serviceID:   t.ServiceID,
				specVersion: *t.SpecVersion,
			}

			if tasksByCommonSpec[taskGroupKey] == nil {
				tasksByCommonSpec[taskGroupKey] = make(map[string]*api.Task)
			}
			tasksByCommonSpec[taskGroupKey][taskID] = t
		} else {
			// This task doesn't have a spec version. We have to
			// schedule it as a one-off.
			oneOffTasks = append(oneOffTasks, t)
		}
		delete(s.unassignedTasks, taskID)
	}

	for _, taskGroup := range tasksByCommonSpec {
		s.scheduleTaskGroup(ctx, taskGroup, schedulingDecisions)
	}
	for _, t := range oneOffTasks {
		s.scheduleTaskGroup(ctx, map[string]*api.Task{t.ID: t}, schedulingDecisions)
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
	err := s.store.Batch(func(batch *store.Batch) error {
		for len(schedulingDecisions) > 0 {
			err := batch.Update(func(tx store.Tx) error {
				// Update exactly one task inside this Update
				// callback.
				for taskID, decision := range schedulingDecisions {
					delete(schedulingDecisions, taskID)

					t := store.GetTask(tx, taskID)
					if t == nil {
						// Task no longer exists
						s.deleteTask(decision.new)
						continue
					}

					if t.Status.State == decision.new.Status.State &&
						t.Status.Message == decision.new.Status.Message &&
						t.Status.Err == decision.new.Status.Err {
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
		failed = append(failed, successful...)
		successful = nil
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
		newT.Status.Err = s.pipeline.Explain()
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
		recentFailuresA := a.countRecentFailures(now, t)
		recentFailuresB := b.countRecentFailures(now, t)

		if recentFailuresA >= maxFailures || recentFailuresB >= maxFailures {
			if recentFailuresA > recentFailuresB {
				return false
			}
			if recentFailuresB > recentFailuresA {
				return true
			}
		}

		tasksByServiceA := a.ActiveTasksCountByService[t.ServiceID]
		tasksByServiceB := b.ActiveTasksCountByService[t.ServiceID]

		if tasksByServiceA < tasksByServiceB {
			return true
		}
		if tasksByServiceA > tasksByServiceB {
			return false
		}

		// Total number of tasks breaks ties.
		return a.ActiveTasksCount < b.ActiveTasksCount
	}

	var prefs []*api.PlacementPreference
	if t.Spec.Placement != nil {
		prefs = t.Spec.Placement.Preferences
	}

	tree := s.nodeSet.tree(t.ServiceID, prefs, len(taskGroup), s.pipeline.Process, nodeLess)

	s.scheduleNTasksOnSubtree(ctx, len(taskGroup), taskGroup, &tree, schedulingDecisions, nodeLess)
	if len(taskGroup) != 0 {
		s.noSuitableNode(ctx, taskGroup, schedulingDecisions)
	}
}

func (s *Scheduler) scheduleNTasksOnSubtree(ctx context.Context, n int, taskGroup map[string]*api.Task, tree *decisionTree, schedulingDecisions map[string]schedulingDecision, nodeLess func(a *NodeInfo, b *NodeInfo) bool) int {
	if tree.next == nil {
		nodes := tree.orderedNodes(s.pipeline.Process, nodeLess)
		if len(nodes) == 0 {
			return 0
		}

		return s.scheduleNTasksOnNodes(ctx, n, taskGroup, nodes, schedulingDecisions, nodeLess)
	}

	// Walk the tree and figure out how the tasks should be split at each
	// level.
	tasksScheduled := 0
	tasksInUsableBranches := tree.tasks
	var noRoom map[*decisionTree]struct{}

	// Try to make branches even until either all branches are
	// full, or all tasks have been scheduled.
	for tasksScheduled != n && len(noRoom) != len(tree.next) {
		desiredTasksPerBranch := (tasksInUsableBranches + n - tasksScheduled) / (len(tree.next) - len(noRoom))
		remainder := (tasksInUsableBranches + n - tasksScheduled) % (len(tree.next) - len(noRoom))

		for _, subtree := range tree.next {
			if noRoom != nil {
				if _, ok := noRoom[subtree]; ok {
					continue
				}
			}
			subtreeTasks := subtree.tasks
			if subtreeTasks < desiredTasksPerBranch || (subtreeTasks == desiredTasksPerBranch && remainder > 0) {
				tasksToAssign := desiredTasksPerBranch - subtreeTasks
				if remainder > 0 {
					tasksToAssign++
				}
				res := s.scheduleNTasksOnSubtree(ctx, tasksToAssign, taskGroup, subtree, schedulingDecisions, nodeLess)
				if res < tasksToAssign {
					if noRoom == nil {
						noRoom = make(map[*decisionTree]struct{})
					}
					noRoom[subtree] = struct{}{}
					tasksInUsableBranches -= subtreeTasks
				} else if remainder > 0 {
					remainder--
				}
				tasksScheduled += res
			}
		}
	}

	return tasksScheduled
}

func (s *Scheduler) scheduleNTasksOnNodes(ctx context.Context, n int, taskGroup map[string]*api.Task, nodes []NodeInfo, schedulingDecisions map[string]schedulingDecision, nodeLess func(a *NodeInfo, b *NodeInfo) bool) int {
	tasksScheduled := 0
	failedConstraints := make(map[int]bool) // key is index in nodes slice
	nodeIter := 0
	nodeCount := len(nodes)
	for taskID, t := range taskGroup {
		// Skip tasks which were already scheduled because they ended
		// up in two groups at once.
		if _, exists := schedulingDecisions[taskID]; exists {
			continue
		}

		node := &nodes[nodeIter%nodeCount]

		log.G(ctx).WithField("task.id", t.ID).Debugf("assigning to node %s", node.ID)
		newT := *t
		newT.NodeID = node.ID
		newT.Status = api.TaskStatus{
			State:     api.TaskStateAssigned,
			Timestamp: ptypes.MustTimestampProto(time.Now()),
			Message:   "scheduler assigned task to node",
		}
		s.allTasks[t.ID] = &newT

		nodeInfo, err := s.nodeSet.nodeInfo(node.ID)
		if err == nil && nodeInfo.addTask(&newT) {
			s.nodeSet.updateNode(nodeInfo)
			nodes[nodeIter%nodeCount] = nodeInfo
		}

		schedulingDecisions[taskID] = schedulingDecision{old: t, new: &newT}
		delete(taskGroup, taskID)
		tasksScheduled++
		if tasksScheduled == n {
			return tasksScheduled
		}

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
				return tasksScheduled
			}
		}
	}

	return tasksScheduled
}

func (s *Scheduler) noSuitableNode(ctx context.Context, taskGroup map[string]*api.Task, schedulingDecisions map[string]schedulingDecision) {
	explanation := s.pipeline.Explain()
	for _, t := range taskGroup {
		log.G(ctx).WithField("task.id", t.ID).Debug("no suitable node available for task")

		newT := *t
		newT.Status.Timestamp = ptypes.MustTimestampProto(time.Now())
		if explanation != "" {
			newT.Status.Err = "no suitable node (" + explanation + ")"
		} else {
			newT.Status.Err = "no suitable node"
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
