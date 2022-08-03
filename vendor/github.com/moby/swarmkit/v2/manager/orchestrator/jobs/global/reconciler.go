package global

import (
	"context"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/manager/constraint"
	"github.com/moby/swarmkit/v2/manager/orchestrator"
	"github.com/moby/swarmkit/v2/manager/state/store"
)

// restartSupervisor is an interface representing the methods from the
// restart.SupervisorInterface that are actually needed by the reconciler. This
// more limited interface allows us to write a less ugly fake for unit testing.
type restartSupervisor interface {
	Restart(context.Context, store.Tx, *api.Cluster, *api.Service, api.Task) error
}

// Reconciler is an object that manages reconciliation of global jobs. It is
// blocking and non-asynchronous, for ease of testing. It implements the
// Reconciler interface from the orchestrator package above it, and the
// taskinit.InitHandler interface.
type Reconciler struct {
	store *store.MemoryStore

	restart restartSupervisor
}

// NewReconciler creates a new global job reconciler.
func NewReconciler(store *store.MemoryStore, restart restartSupervisor) *Reconciler {
	return &Reconciler{
		store:   store,
		restart: restart,
	}
}

// ReconcileService reconciles one global job service.
func (r *Reconciler) ReconcileService(id string) error {
	var (
		service *api.Service
		cluster *api.Cluster
		tasks   []*api.Task
		nodes   []*api.Node
		viewErr error
	)

	// we need to first get the latest iteration of the service, its tasks, and
	// the nodes in the cluster.
	r.store.View(func(tx store.ReadTx) {
		service = store.GetService(tx, id)
		if service == nil {
			return
		}

		// getting tasks with FindTasks should only return an error if we've
		// made a mistake coding; there's no user-input or even reasonable
		// system state that can cause it. If it returns an error, we'll just
		// panic and crash.
		tasks, viewErr = store.FindTasks(tx, store.ByServiceID(id))
		if viewErr != nil {
			return
		}

		// same as with FindTasks
		nodes, viewErr = store.FindNodes(tx, store.All)
		if viewErr != nil {
			return
		}

		clusters, _ := store.FindClusters(tx, store.All)
		if len(clusters) == 1 {
			cluster = clusters[0]
		} else if len(clusters) > 1 {
			panic("there should never be more than one cluster object")
		}
	})

	if viewErr != nil {
		return viewErr
	}

	// the service may be nil if the service has been deleted before we entered
	// the View.
	if service == nil {
		return nil
	}

	if service.JobStatus == nil {
		service.JobStatus = &api.JobStatus{}
	}

	// we need to compute the constraints on the service so we know which nodes
	// to schedule it on
	var constraints []constraint.Constraint
	if service.Spec.Task.Placement != nil && len(service.Spec.Task.Placement.Constraints) != 0 {
		// constraint.Parse does return an error, but we don't need to check
		// it, because it was already checked when the service was created or
		// updated.
		constraints, _ = constraint.Parse(service.Spec.Task.Placement.Constraints)
	}

	var candidateNodes []string
	var invalidNodes []string
	for _, node := range nodes {
		// instead of having a big ugly multi-line boolean expression in the
		// if-statement, we'll have several if-statements, and bail out of
		// this loop iteration with continue if the node is not acceptable
		if !constraint.NodeMatches(constraints, node) {
			continue
		}

		// if a node is invalid, we should remove any tasks that might be on it
		if orchestrator.InvalidNode(node) {
			invalidNodes = append(invalidNodes, node.ID)
			continue
		}

		if node.Spec.Availability != api.NodeAvailabilityActive {
			continue
		}
		if node.Status.State != api.NodeStatus_READY {
			continue
		}
		// you can append to a nil slice and get a non-nil slice, which is
		// pretty slick.
		candidateNodes = append(candidateNodes, node.ID)
	}

	// now, we have a list of all nodes that match constraints. it's time to
	// match running tasks to the nodes. we need to identify all nodes that
	// need new tasks, which is any node that doesn't have a task of this job
	// iteration. trade some space for some time by building a node ID to task
	// ID mapping, so that we're just doing 2x linear operation, instead of a
	// quadratic operation.
	nodeToTask := map[string]string{}
	// additionally, while we're iterating through tasks, if any of those tasks
	// are failed, we'll hand them to the restart supervisor to handle
	restartTasks := []string{}
	// and if there are any tasks belonging to old job iterations, set them to
	// be removed
	removeTasks := []string{}
	for _, task := range tasks {
		// match all tasks belonging to this job iteration which are in desired
		// state completed, including failed tasks. We only want to create
		// tasks for nodes on which there are no existing tasks.
		if task.JobIteration != nil {
			if task.JobIteration.Index == service.JobStatus.JobIteration.Index &&
				task.DesiredState <= api.TaskStateCompleted {
				// we already know the task is desired to be executing (because its
				// desired state is Completed). Check here to see if it's already
				// failed, so we can restart it
				if task.Status.State > api.TaskStateCompleted {
					restartTasks = append(restartTasks, task.ID)
				}
				nodeToTask[task.NodeID] = task.ID
			}

			if task.JobIteration.Index != service.JobStatus.JobIteration.Index {
				if task.DesiredState != api.TaskStateRemove {
					removeTasks = append(removeTasks, task.ID)
				}
			}
		}
	}

	return r.store.Batch(func(batch *store.Batch) error {
		// first, create any new tasks required.
		for _, node := range candidateNodes {
			// check if there is a task for this node ID. If not, then we need
			// to create one.
			if _, ok := nodeToTask[node]; !ok {
				if err := batch.Update(func(tx store.Tx) error {
					// if the node does not already have a running or completed
					// task, create a task for this node.
					task := orchestrator.NewTask(cluster, service, 0, node)
					task.JobIteration = &service.JobStatus.JobIteration
					task.DesiredState = api.TaskStateCompleted
					return store.CreateTask(tx, task)
				}); err != nil {
					return err
				}
			}
		}

		// then, restart any tasks that are failed
		for _, taskID := range restartTasks {
			if err := batch.Update(func(tx store.Tx) error {
				// get the latest version of the task for the restart
				t := store.GetTask(tx, taskID)
				// if it's deleted, nothing to do
				if t == nil {
					return nil
				}

				// if it's not still desired to be running, then don't restart
				// it.
				if t.DesiredState > api.TaskStateCompleted {
					return nil
				}

				// Finally, restart it
				// TODO(dperny): pass in context to ReconcileService, so we can
				// pass it in here.
				return r.restart.Restart(context.Background(), tx, cluster, service, *t)
			}); err != nil {
				// TODO(dperny): probably should log like in the other
				// orchestrators instead of returning here.
				return err
			}
		}

		// remove tasks that need to be removed
		for _, taskID := range removeTasks {
			if err := batch.Update(func(tx store.Tx) error {
				t := store.GetTask(tx, taskID)
				if t == nil {
					return nil
				}

				if t.DesiredState == api.TaskStateRemove {
					return nil
				}

				t.DesiredState = api.TaskStateRemove
				return store.UpdateTask(tx, t)
			}); err != nil {
				return err
			}
		}

		// finally, shut down any tasks on invalid nodes
		for _, nodeID := range invalidNodes {
			if taskID, ok := nodeToTask[nodeID]; ok {
				if err := batch.Update(func(tx store.Tx) error {
					t := store.GetTask(tx, taskID)
					if t == nil {
						return nil
					}
					// if the task is still desired to be running, and is still
					// actually, running, then it still needs to be shut down.
					if t.DesiredState > api.TaskStateCompleted || t.Status.State <= api.TaskStateRunning {
						t.DesiredState = api.TaskStateShutdown
						return store.UpdateTask(tx, t)
					}
					return nil
				}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// IsRelatedService returns true if the task is a global job. This method
// fulfills the taskinit.InitHandler interface. Because it is just a wrapper
// around a well-tested function call, it has no tests of its own.
func (r *Reconciler) IsRelatedService(service *api.Service) bool {
	return orchestrator.IsGlobalJob(service)
}

// FixTask validates that a task is compliant with the rest of the cluster
// state, and fixes it if it's not. This covers some main scenarios:
//
//   - The node that the task is running on is now paused or drained. we do not
//     need to check if the node still meets constraints -- that is the purview
//     of the constraint enforcer.
//   - The task has failed and needs to be restarted.
//
// This implements the FixTask method of the taskinit.InitHandler interface.
func (r *Reconciler) FixTask(ctx context.Context, batch *store.Batch, t *api.Task) {
	// tasks already desired to be shut down need no action.
	if t.DesiredState > api.TaskStateCompleted {
		return
	}

	batch.Update(func(tx store.Tx) error {
		node := store.GetNode(tx, t.NodeID)
		// if the node is no longer a valid node for this task, we need to shut
		// it down
		if orchestrator.InvalidNode(node) {
			task := store.GetTask(tx, t.ID)
			if task != nil && task.DesiredState < api.TaskStateShutdown {
				task.DesiredState = api.TaskStateShutdown
				return store.UpdateTask(tx, task)
			}
		}
		// we will reconcile all services after fixing the tasks, so we don't
		// need to restart tasks right now; we'll do so after this.
		return nil
	})
}

// SlotTuple returns a slot tuple representing this task. It implements the
// taskinit.InitHandler interface.
func (r *Reconciler) SlotTuple(t *api.Task) orchestrator.SlotTuple {
	return orchestrator.SlotTuple{
		ServiceID: t.ServiceID,
		NodeID:    t.NodeID,
	}
}
