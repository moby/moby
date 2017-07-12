package global

import (
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/constraint"
	"github.com/docker/swarmkit/manager/orchestrator"
	"github.com/docker/swarmkit/manager/orchestrator/restart"
	"github.com/docker/swarmkit/manager/orchestrator/taskinit"
	"github.com/docker/swarmkit/manager/orchestrator/update"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
)

type globalService struct {
	*api.Service

	// Compiled constraints
	constraints []constraint.Constraint
}

// Orchestrator runs a reconciliation loop to create and destroy tasks as
// necessary for global services.
type Orchestrator struct {
	store *store.MemoryStore
	// nodes is the set of non-drained nodes in the cluster, indexed by node ID
	nodes map[string]*api.Node
	// globalServices has all the global services in the cluster, indexed by ServiceID
	globalServices map[string]globalService
	restartTasks   map[string]struct{}

	// stopChan signals to the state machine to stop running.
	stopChan chan struct{}
	// doneChan is closed when the state machine terminates.
	doneChan chan struct{}

	updater  *update.Supervisor
	restarts *restart.Supervisor

	cluster *api.Cluster // local instance of the cluster
}

// NewGlobalOrchestrator creates a new global Orchestrator
func NewGlobalOrchestrator(store *store.MemoryStore) *Orchestrator {
	restartSupervisor := restart.NewSupervisor(store)
	updater := update.NewSupervisor(store, restartSupervisor)
	return &Orchestrator{
		store:          store,
		nodes:          make(map[string]*api.Node),
		globalServices: make(map[string]globalService),
		stopChan:       make(chan struct{}),
		doneChan:       make(chan struct{}),
		updater:        updater,
		restarts:       restartSupervisor,
		restartTasks:   make(map[string]struct{}),
	}
}

func (g *Orchestrator) initTasks(ctx context.Context, readTx store.ReadTx) error {
	return taskinit.CheckTasks(ctx, g.store, readTx, g, g.restarts)
}

// Run contains the global orchestrator event loop
func (g *Orchestrator) Run(ctx context.Context) error {
	defer close(g.doneChan)

	// Watch changes to services and tasks
	queue := g.store.WatchQueue()
	watcher, cancel := queue.Watch()
	defer cancel()

	// lookup the cluster
	var err error
	g.store.View(func(readTx store.ReadTx) {
		var clusters []*api.Cluster
		clusters, err = store.FindClusters(readTx, store.ByName("default"))

		if len(clusters) != 1 {
			return // just pick up the cluster when it is created.
		}
		g.cluster = clusters[0]
	})
	if err != nil {
		return err
	}

	// Get list of nodes
	var nodes []*api.Node
	g.store.View(func(readTx store.ReadTx) {
		nodes, err = store.FindNodes(readTx, store.All)
	})
	if err != nil {
		return err
	}
	for _, n := range nodes {
		g.updateNode(n)
	}

	// Lookup global services
	var existingServices []*api.Service
	g.store.View(func(readTx store.ReadTx) {
		existingServices, err = store.FindServices(readTx, store.All)
	})
	if err != nil {
		return err
	}

	var reconcileServiceIDs []string
	for _, s := range existingServices {
		if orchestrator.IsGlobalService(s) {
			g.updateService(s)
			reconcileServiceIDs = append(reconcileServiceIDs, s.ID)
		}
	}

	// fix tasks in store before reconciliation loop
	g.store.View(func(readTx store.ReadTx) {
		err = g.initTasks(ctx, readTx)
	})
	if err != nil {
		return err
	}

	g.tickTasks(ctx)
	g.reconcileServices(ctx, reconcileServiceIDs)

	for {
		select {
		case event := <-watcher:
			// TODO(stevvooe): Use ctx to limit running time of operation.
			switch v := event.(type) {
			case api.EventUpdateCluster:
				g.cluster = v.Cluster
			case api.EventCreateService:
				if !orchestrator.IsGlobalService(v.Service) {
					continue
				}
				g.updateService(v.Service)
				g.reconcileServices(ctx, []string{v.Service.ID})
			case api.EventUpdateService:
				if !orchestrator.IsGlobalService(v.Service) {
					continue
				}
				g.updateService(v.Service)
				g.reconcileServices(ctx, []string{v.Service.ID})
			case api.EventDeleteService:
				if !orchestrator.IsGlobalService(v.Service) {
					continue
				}
				orchestrator.DeleteServiceTasks(ctx, g.store, v.Service)
				// delete the service from service map
				delete(g.globalServices, v.Service.ID)
				g.restarts.ClearServiceHistory(v.Service.ID)
			case api.EventCreateNode:
				g.updateNode(v.Node)
				g.reconcileOneNode(ctx, v.Node)
			case api.EventUpdateNode:
				g.updateNode(v.Node)
				switch v.Node.Status.State {
				// NodeStatus_DISCONNECTED is a transient state, no need to make any change
				case api.NodeStatus_DOWN:
					g.foreachTaskFromNode(ctx, v.Node, g.shutdownTask)
				case api.NodeStatus_READY:
					// node could come back to READY from DOWN or DISCONNECT
					g.reconcileOneNode(ctx, v.Node)
				}
			case api.EventDeleteNode:
				g.foreachTaskFromNode(ctx, v.Node, g.deleteTask)
				delete(g.nodes, v.Node.ID)
			case api.EventUpdateTask:
				g.handleTaskChange(ctx, v.Task)
			case api.EventDeleteTask:
				// CLI allows deleting task
				if _, exists := g.globalServices[v.Task.ServiceID]; !exists {
					continue
				}
				g.reconcileServicesOneNode(ctx, []string{v.Task.ServiceID}, v.Task.NodeID)
			}
		case <-g.stopChan:
			return nil
		}
		g.tickTasks(ctx)
	}
}

// FixTask validates a task with the current cluster settings, and takes
// action to make it conformant to node state and service constraint
// it's called at orchestrator initialization
func (g *Orchestrator) FixTask(ctx context.Context, batch *store.Batch, t *api.Task) {
	if _, exists := g.globalServices[t.ServiceID]; !exists {
		return
	}
	// if a task's DesiredState has past running, the task has been processed
	if t.DesiredState > api.TaskStateRunning {
		return
	}

	var node *api.Node
	if t.NodeID != "" {
		node = g.nodes[t.NodeID]
	}
	// if the node no longer valid, remove the task
	if t.NodeID == "" || orchestrator.InvalidNode(node) {
		g.shutdownTask(ctx, batch, t)
		return
	}

	// restart a task if it fails
	if t.Status.State > api.TaskStateRunning {
		g.restartTasks[t.ID] = struct{}{}
	}
}

// handleTaskChange defines what orchestrator does when a task is updated by agent
func (g *Orchestrator) handleTaskChange(ctx context.Context, t *api.Task) {
	if _, exists := g.globalServices[t.ServiceID]; !exists {
		return
	}
	// if a task's DesiredState has past running, which
	// means the task has been processed
	if t.DesiredState > api.TaskStateRunning {
		return
	}

	// if a task has passed running, restart it
	if t.Status.State > api.TaskStateRunning {
		g.restartTasks[t.ID] = struct{}{}
	}
}

// Stop stops the orchestrator.
func (g *Orchestrator) Stop() {
	close(g.stopChan)
	<-g.doneChan
	g.updater.CancelAll()
	g.restarts.CancelAll()
}

func (g *Orchestrator) foreachTaskFromNode(ctx context.Context, node *api.Node, cb func(context.Context, *store.Batch, *api.Task)) {
	var (
		tasks []*api.Task
		err   error
	)
	g.store.View(func(tx store.ReadTx) {
		tasks, err = store.FindTasks(tx, store.ByNodeID(node.ID))
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("global orchestrator: foreachTaskFromNode failed finding tasks")
		return
	}

	err = g.store.Batch(func(batch *store.Batch) error {
		for _, t := range tasks {
			// Global orchestrator only removes tasks from globalServices
			if _, exists := g.globalServices[t.ServiceID]; exists {
				cb(ctx, batch, t)
			}
		}
		return nil
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("global orchestrator: foreachTaskFromNode failed batching tasks")
	}
}

func (g *Orchestrator) reconcileServices(ctx context.Context, serviceIDs []string) {
	nodeCompleted := make(map[string]map[string]struct{})
	nodeTasks := make(map[string]map[string][]*api.Task)

	g.store.View(func(tx store.ReadTx) {
		for _, serviceID := range serviceIDs {
			tasks, err := store.FindTasks(tx, store.ByServiceID(serviceID))
			if err != nil {
				log.G(ctx).WithError(err).Errorf("global orchestrator: reconcileServices failed finding tasks for service %s", serviceID)
				continue
			}

			// a node may have completed this service
			nodeCompleted[serviceID] = make(map[string]struct{})
			// nodeID -> task list
			nodeTasks[serviceID] = make(map[string][]*api.Task)

			for _, t := range tasks {
				if t.DesiredState <= api.TaskStateRunning {
					// Collect all running instances of this service
					nodeTasks[serviceID][t.NodeID] = append(nodeTasks[serviceID][t.NodeID], t)
				} else {
					// for finished tasks, check restartPolicy
					if isTaskCompleted(t, orchestrator.RestartCondition(t)) {
						nodeCompleted[serviceID][t.NodeID] = struct{}{}
					}
				}
			}
		}
	})

	updates := make(map[*api.Service][]orchestrator.Slot)

	err := g.store.Batch(func(batch *store.Batch) error {
		for _, serviceID := range serviceIDs {
			var updateTasks []orchestrator.Slot

			if _, exists := nodeTasks[serviceID]; !exists {
				continue
			}

			service := g.globalServices[serviceID]

			for nodeID, node := range g.nodes {
				meetsConstraints := constraint.NodeMatches(service.constraints, node)
				ntasks := nodeTasks[serviceID][nodeID]
				delete(nodeTasks[serviceID], nodeID)

				// if restart policy considers this node has finished its task
				// it should remove all running tasks
				if _, exists := nodeCompleted[serviceID][nodeID]; exists || !meetsConstraints {
					g.shutdownTasks(ctx, batch, ntasks)
					continue
				}

				if node.Spec.Availability == api.NodeAvailabilityPause {
					// the node is paused, so we won't add or update
					// any tasks
					continue
				}

				// this node needs to run 1 copy of the task
				if len(ntasks) == 0 {
					g.addTask(ctx, batch, service.Service, nodeID)
				} else {
					updateTasks = append(updateTasks, ntasks)
				}
			}

			if len(updateTasks) > 0 {
				updates[service.Service] = updateTasks
			}

			// Remove any tasks assigned to nodes not found in g.nodes.
			// These must be associated with nodes that are drained, or
			// nodes that no longer exist.
			for _, ntasks := range nodeTasks[serviceID] {
				g.shutdownTasks(ctx, batch, ntasks)
			}
		}
		return nil
	})

	if err != nil {
		log.G(ctx).WithError(err).Errorf("global orchestrator: reconcileServices transaction failed")
	}

	for service, updateTasks := range updates {
		g.updater.Update(ctx, g.cluster, service, updateTasks)
	}
}

// updateNode updates g.nodes based on the current node value
func (g *Orchestrator) updateNode(node *api.Node) {
	if node.Spec.Availability == api.NodeAvailabilityDrain {
		delete(g.nodes, node.ID)
	} else {
		g.nodes[node.ID] = node
	}
}

// updateService updates g.globalServices based on the current service value
func (g *Orchestrator) updateService(service *api.Service) {
	var constraints []constraint.Constraint

	if service.Spec.Task.Placement != nil && len(service.Spec.Task.Placement.Constraints) != 0 {
		constraints, _ = constraint.Parse(service.Spec.Task.Placement.Constraints)
	}

	g.globalServices[service.ID] = globalService{
		Service:     service,
		constraints: constraints,
	}
}

// reconcileOneNode checks all global services on one node
func (g *Orchestrator) reconcileOneNode(ctx context.Context, node *api.Node) {
	if node.Spec.Availability == api.NodeAvailabilityDrain {
		log.G(ctx).Debugf("global orchestrator: node %s in drain state, removing tasks from it", node.ID)
		g.foreachTaskFromNode(ctx, node, g.shutdownTask)
		return
	}

	var serviceIDs []string
	for id := range g.globalServices {
		serviceIDs = append(serviceIDs, id)
	}
	g.reconcileServicesOneNode(ctx, serviceIDs, node.ID)
}

// reconcileServicesOneNode checks the specified services on one node
func (g *Orchestrator) reconcileServicesOneNode(ctx context.Context, serviceIDs []string, nodeID string) {
	node, exists := g.nodes[nodeID]
	if !exists {
		return
	}

	// whether each service has completed on the node
	completed := make(map[string]bool)
	// tasks by service
	tasks := make(map[string][]*api.Task)

	var (
		tasksOnNode []*api.Task
		err         error
	)

	g.store.View(func(tx store.ReadTx) {
		tasksOnNode, err = store.FindTasks(tx, store.ByNodeID(nodeID))
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("global orchestrator: reconcile failed finding tasks on node %s", nodeID)
		return
	}

	for _, serviceID := range serviceIDs {
		for _, t := range tasksOnNode {
			if t.ServiceID != serviceID {
				continue
			}
			if t.DesiredState <= api.TaskStateRunning {
				tasks[serviceID] = append(tasks[serviceID], t)
			} else {
				if isTaskCompleted(t, orchestrator.RestartCondition(t)) {
					completed[serviceID] = true
				}
			}
		}
	}

	err = g.store.Batch(func(batch *store.Batch) error {
		for _, serviceID := range serviceIDs {
			service, exists := g.globalServices[serviceID]
			if !exists {
				continue
			}

			if !constraint.NodeMatches(service.constraints, node) {
				continue
			}

			// if restart policy considers this node has finished its task
			// it should remove all running tasks
			if completed[serviceID] {
				g.shutdownTasks(ctx, batch, tasks[serviceID])
				continue
			}

			if node.Spec.Availability == api.NodeAvailabilityPause {
				// the node is paused, so we won't add or update tasks
				continue
			}

			if len(tasks) == 0 {
				g.addTask(ctx, batch, service.Service, nodeID)
			} else {
				// If task is out of date, update it. This can happen
				// on node reconciliation if, for example, we pause a
				// node, update the service, and then activate the node
				// later.

				// We don't use g.updater here for two reasons:
				// - This is not a rolling update. Since it was not
				//   triggered directly by updating the service, it
				//   should not observe the rolling update parameters
				//   or show status in UpdateStatus.
				// - Calling Update cancels any current rolling updates
				//   for the service, such as one triggered by service
				//   reconciliation.

				var (
					dirtyTasks []*api.Task
					cleanTasks []*api.Task
				)

				for _, t := range tasks[serviceID] {
					if orchestrator.IsTaskDirty(service.Service, t) {
						dirtyTasks = append(dirtyTasks, t)
					} else {
						cleanTasks = append(cleanTasks, t)
					}
				}

				if len(cleanTasks) == 0 {
					g.addTask(ctx, batch, service.Service, nodeID)
				} else {
					dirtyTasks = append(dirtyTasks, cleanTasks[1:]...)
				}
				g.shutdownTasks(ctx, batch, dirtyTasks)
			}
		}
		return nil
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("global orchestrator: reconcileServiceOneNode batch failed")
	}
}

func (g *Orchestrator) tickTasks(ctx context.Context) {
	if len(g.restartTasks) == 0 {
		return
	}
	err := g.store.Batch(func(batch *store.Batch) error {
		for taskID := range g.restartTasks {
			err := batch.Update(func(tx store.Tx) error {
				t := store.GetTask(tx, taskID)
				if t == nil || t.DesiredState > api.TaskStateRunning {
					return nil
				}

				service := store.GetService(tx, t.ServiceID)
				if service == nil {
					return nil
				}

				node, nodeExists := g.nodes[t.NodeID]
				serviceEntry, serviceExists := g.globalServices[t.ServiceID]
				if !nodeExists || !serviceExists {
					return nil
				}
				if !constraint.NodeMatches(serviceEntry.constraints, node) {
					t.DesiredState = api.TaskStateShutdown
					return store.UpdateTask(tx, t)
				}

				return g.restarts.Restart(ctx, tx, g.cluster, service, *t)
			})
			if err != nil {
				log.G(ctx).WithError(err).Errorf("orchestrator restartTask transaction failed")
			}
		}
		return nil
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("global orchestrator: restartTask transaction failed")
	}
	g.restartTasks = make(map[string]struct{})
}

func (g *Orchestrator) shutdownTask(ctx context.Context, batch *store.Batch, t *api.Task) {
	// set existing task DesiredState to TaskStateShutdown
	// TODO(aaronl): optimistic update?
	err := batch.Update(func(tx store.Tx) error {
		t = store.GetTask(tx, t.ID)
		if t != nil && t.DesiredState < api.TaskStateShutdown {
			t.DesiredState = api.TaskStateShutdown
			return store.UpdateTask(tx, t)
		}
		return nil
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("global orchestrator: shutdownTask failed to shut down %s", t.ID)
	}
}

func (g *Orchestrator) addTask(ctx context.Context, batch *store.Batch, service *api.Service, nodeID string) {
	task := orchestrator.NewTask(g.cluster, service, 0, nodeID)

	err := batch.Update(func(tx store.Tx) error {
		if store.GetService(tx, service.ID) == nil {
			return nil
		}
		return store.CreateTask(tx, task)
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("global orchestrator: failed to create task")
	}
}

func (g *Orchestrator) shutdownTasks(ctx context.Context, batch *store.Batch, tasks []*api.Task) {
	for _, t := range tasks {
		g.shutdownTask(ctx, batch, t)
	}
}

func (g *Orchestrator) deleteTask(ctx context.Context, batch *store.Batch, t *api.Task) {
	err := batch.Update(func(tx store.Tx) error {
		return store.DeleteTask(tx, t.ID)
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("global orchestrator: deleteTask failed to delete %s", t.ID)
	}
}

// IsRelatedService returns true if the service should be governed by this orchestrator
func (g *Orchestrator) IsRelatedService(service *api.Service) bool {
	return orchestrator.IsGlobalService(service)
}

func isTaskCompleted(t *api.Task, restartPolicy api.RestartPolicy_RestartCondition) bool {
	if t == nil || t.DesiredState <= api.TaskStateRunning {
		return false
	}
	return restartPolicy == api.RestartOnNone ||
		(restartPolicy == api.RestartOnFailure && t.Status.State == api.TaskStateCompleted)
}
