package replicated

import (
	"context"
	"fmt"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/manager/orchestrator"
	"github.com/moby/swarmkit/v2/manager/state/store"
)

// restartSupervisor is an interface representing the methods from the
// restart.SupervisorInterface that are actually needed by the reconciler. This
// more limited interface allows us to write a less ugly fake for unit testing.
type restartSupervisor interface {
	Restart(context.Context, store.Tx, *api.Cluster, *api.Service, api.Task) error
}

// Reconciler is an object that manages reconciliation of replicated jobs. It
// is blocking and non-asynchronous, for ease of testing. It implements two
// interfaces. The first is the Reconciler interface of the Orchestrator
// package above this one. The second is the taskinit.InitHandler interface.
type Reconciler struct {
	// we need the store, of course, to do updates
	store *store.MemoryStore

	restart restartSupervisor
}

// newReconciler creates a new reconciler object
func NewReconciler(store *store.MemoryStore, restart restartSupervisor) *Reconciler {
	return &Reconciler{
		store:   store,
		restart: restart,
	}
}

// ReconcileService reconciles the replicated job service with the given ID by
// checking to see if new replicas should be created. reconcileService returns
// an error if there is some case prevent it from correctly reconciling the
// service.
func (r *Reconciler) ReconcileService(id string) error {
	var (
		service *api.Service
		tasks   []*api.Task
		cluster *api.Cluster
		viewErr error
	)
	// first, get the service and all of its tasks
	r.store.View(func(tx store.ReadTx) {
		service = store.GetService(tx, id)

		tasks, viewErr = store.FindTasks(tx, store.ByServiceID(id))

		// there should only ever be 1 cluster object, but for reasons
		// forgotten by me, it needs to be retrieved in a rather roundabout way
		// from the store
		var clusters []*api.Cluster
		clusters, viewErr = store.FindClusters(tx, store.All)
		if len(clusters) == 1 {
			cluster = clusters[0]
		} else if len(clusters) > 1 {
			// this should never happen, and indicates that the system is
			// broken.
			panic("there should never be more than one cluster object")
		}
	})

	// errors during view should only happen in a few rather catastrophic
	// cases, but here it's not unreasonable to just return an error anyway.
	if viewErr != nil {
		return viewErr
	}

	// if the service has already been deleted, there's nothing to do here.
	if service == nil {
		return nil
	}

	// if this is the first iteration of the service, it may not yet have a
	// JobStatus, so we should create one if so. this won't actually be
	// committed, though.
	if service.JobStatus == nil {
		service.JobStatus = &api.JobStatus{}
	}

	// Jobs can be run in multiple iterations. The JobStatus of the service
	// indicates which Version of iteration we're on. We should only be looking
	// at tasks of the latest Version

	jobVersion := service.JobStatus.JobIteration.Index

	// now, check how many tasks we need and how many we have running. note
	// that some of these Running tasks may complete before we even finish this
	// code block, and so we might have to immediately re-enter reconciliation,
	// so this number is 100% definitive, but it is accurate for this
	// particular moment in time, and it won't result in us going OVER the
	// needed task count
	//
	// importantly, we are computing only how many _new_ tasks are needed. Some
	// tasks may need to be restarted as well, but we don't do this directly;
	// restarting tasks is under the purview of the restartSupervisor.
	//
	// also also, for the math later, we need these values to be of type uint64.
	runningTasks := uint64(0)
	completeTasks := uint64(0)
	restartTasks := []string{}
	removeTasks := []string{}

	// for replicated jobs, each task will get a different slot number, so that
	// when the job has completed, there will be one Completed task in every
	// slot number [0, TotalCompletions-1].
	//
	// By assigning each task to a unique slot, we simply handling of
	// restarting failed tasks through the restart manager.
	slots := map[uint64]bool{}
	for _, task := range tasks {
		// we only care about tasks from this job iteration. tasks from the
		// previous job iteration are not important
		if task.JobIteration != nil {
			if task.JobIteration.Index == jobVersion {
				if task.Status.State == api.TaskStateCompleted {
					completeTasks++
					slots[task.Slot] = true
				}

				// the Restart Manager may put a task in the desired state Ready,
				// so we should match not only tasks in desired state Completed,
				// but also those in any valid running state.
				if task.Status.State != api.TaskStateCompleted && task.DesiredState <= api.TaskStateCompleted {
					runningTasks++
					slots[task.Slot] = true

					// if the task is in a terminal state, we might need to restart
					// it. throw it on the pile if so. this is still counted as a
					// running task for the purpose of determining how many new
					// tasks to create.
					if task.Status.State > api.TaskStateCompleted {
						restartTasks = append(restartTasks, task.ID)
					}
				}
			} else {
				// tasks belonging to a previous iteration of the job may
				// exist. if any such tasks exist, they should have their task
				// state set to Remove
				if task.Status.State <= api.TaskStateRunning && task.DesiredState != api.TaskStateRemove {
					removeTasks = append(removeTasks, task.ID)
				}
			}
		}
	}

	// now that we have our counts, we need to see how many new tasks to
	// create. this number can never exceed MaxConcurrent, but also should not
	// result in us exceeding TotalCompletions. first, get these numbers out of
	// the service spec.
	rj := service.Spec.GetReplicatedJob()

	// possibleNewTasks gives us the upper bound for how many tasks we'll
	// create. also, ugh, subtracting uints. there's no way this can ever go
	// wrong.
	possibleNewTasks := rj.MaxConcurrent - runningTasks

	// allowedNewTasks is how many tasks we could create, if there were no
	// restriction on maximum concurrency. This is the total number of tasks
	// we want completed, minus the tasks that are already completed, minus
	// the tasks that are in progress.
	//
	// seriously, ugh, subtracting unsigned ints. totally a fine and not at all
	// risky operation, with no possibility for catastrophe
	allowedNewTasks := rj.TotalCompletions - completeTasks - runningTasks

	// the lower number of allowedNewTasks and possibleNewTasks is how many we
	// can create. we'll just use an if statement instead of some fancy floor
	// function.
	actualNewTasks := allowedNewTasks
	if possibleNewTasks < allowedNewTasks {
		actualNewTasks = possibleNewTasks
	}

	// this check might seem odd, but it protects us from an underflow of the
	// above subtractions, which, again, is a totally impossible thing that can
	// never happen, ever, obviously.
	if actualNewTasks > rj.TotalCompletions {
		return fmt.Errorf(
			"uint64 underflow, we're not going to create %v tasks",
			actualNewTasks,
		)
	}

	// finally, we can create these tasks. do this in a batch operation, to
	// avoid exceeding transaction size limits
	err := r.store.Batch(func(batch *store.Batch) error {
		for i := uint64(0); i < actualNewTasks; i++ {
			if err := batch.Update(func(tx store.Tx) error {
				var slot uint64
				// each task will go into a unique slot, and at the end, there
				// should be the same number of slots as there are desired
				// total completions. We could simplify this logic by simply
				// assuming that slots are filled in order, but it's a more
				// robust solution to not assume that, and instead assure that
				// the slot is unoccupied.
				for s := uint64(0); s < rj.TotalCompletions; s++ {
					// when we're iterating through, if the service has slots
					// that haven't been used yet (for example, if this is the
					// first time we're running this iteration), then doing
					// a map lookup for the number will return the 0-value
					// (false) even if the number doesn't exist in the map.
					if !slots[s] {
						slot = s
						// once we've found a slot, mark it as occupied, so we
						// don't double assign in subsequent iterations.
						slots[slot] = true
						break
					}
				}

				task := orchestrator.NewTask(cluster, service, slot, "")
				// when we create the task, we also need to set the
				// JobIteration.
				task.JobIteration = &api.Version{Index: jobVersion}
				task.DesiredState = api.TaskStateCompleted

				// finally, create the task in the store.
				return store.CreateTask(tx, task)
			}); err != nil {
				return err
			}
		}

		for _, taskID := range restartTasks {
			if err := batch.Update(func(tx store.Tx) error {
				t := store.GetTask(tx, taskID)
				if t == nil {
					return nil
				}

				if t.DesiredState > api.TaskStateCompleted {
					return nil
				}

				// TODO(dperny): pass in context from above
				return r.restart.Restart(context.Background(), tx, cluster, service, *t)
			}); err != nil {
				return err
			}
		}

		for _, taskID := range removeTasks {
			if err := batch.Update(func(tx store.Tx) error {
				t := store.GetTask(tx, taskID)
				if t == nil {
					return nil
				}

				// don't do unnecessary updates
				if t.DesiredState == api.TaskStateRemove {
					return nil
				}
				t.DesiredState = api.TaskStateRemove
				return store.UpdateTask(tx, t)
			}); err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

// IsRelatedService returns true if the task is a replicated job. This method
// fulfills the taskinit.InitHandler interface. Because it is just a wrapper
// around a well-tested function call, it has no tests of its own.
func (r *Reconciler) IsRelatedService(service *api.Service) bool {
	return orchestrator.IsReplicatedJob(service)
}

// FixTask ostensibly validates that a task is compliant with the rest of the
// cluster state. However, in the replicated jobs case, the only action we
// can take with a noncompliant task is to restart it. Because the replicated
// jobs orchestrator reconciles the whole service at once, any tasks that
// need to be restarted will be done when we make the reconiliation pass over
// all services. Therefore, in this instance, FixTask does nothing except
// implement the FixTask method of the taskinit.InitHandler interface.
func (r *Reconciler) FixTask(_ context.Context, _ *store.Batch, _ *api.Task) {}

// SlotTuple returns an orchestrator.SlotTuple object for this task. It
// implements the taskinit.InitHandler interface
func (r *Reconciler) SlotTuple(t *api.Task) orchestrator.SlotTuple {
	return orchestrator.SlotTuple{
		ServiceID: t.ServiceID,
		Slot:      t.Slot,
	}
}
