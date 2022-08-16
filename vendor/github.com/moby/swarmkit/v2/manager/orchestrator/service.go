package orchestrator

import (
	"context"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/log"
	"github.com/moby/swarmkit/v2/manager/state/store"
)

// IsReplicatedService checks if a service is a replicated service.
func IsReplicatedService(service *api.Service) bool {
	// service nil validation is required as there are scenarios
	// where service is removed from store
	if service == nil {
		return false
	}
	_, ok := service.Spec.GetMode().(*api.ServiceSpec_Replicated)
	return ok
}

// IsGlobalService checks if the service is a global service.
func IsGlobalService(service *api.Service) bool {
	if service == nil {
		return false
	}
	_, ok := service.Spec.GetMode().(*api.ServiceSpec_Global)
	return ok
}

// IsReplicatedJob returns true if the service is a replicated job.
func IsReplicatedJob(service *api.Service) bool {
	if service == nil {
		return false
	}

	_, ok := service.Spec.GetMode().(*api.ServiceSpec_ReplicatedJob)
	return ok
}

// IsGlobalJob returns true if the service is a global job.
func IsGlobalJob(service *api.Service) bool {
	if service == nil {
		return false
	}

	_, ok := service.Spec.GetMode().(*api.ServiceSpec_GlobalJob)
	return ok
}

// SetServiceTasksRemove sets the desired state of tasks associated with a service
// to REMOVE, so that they can be properly shut down by the agent and later removed
// by the task reaper.
func SetServiceTasksRemove(ctx context.Context, s *store.MemoryStore, service *api.Service) {
	var (
		tasks []*api.Task
		err   error
	)
	s.View(func(tx store.ReadTx) {
		tasks, err = store.FindTasks(tx, store.ByServiceID(service.ID))
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("failed to list tasks")
		return
	}

	err = s.Batch(func(batch *store.Batch) error {
		for _, t := range tasks {
			err := batch.Update(func(tx store.Tx) error {
				// the task may have changed for some reason in the meantime
				// since we read it out, so we need to get from the store again
				// within the boundaries of a transaction
				latestTask := store.GetTask(tx, t.ID)

				// time travel is not allowed. if the current desired state is
				// above the one we're trying to go to we can't go backwards.
				// we have nothing to do and we should skip to the next task
				if latestTask.DesiredState > api.TaskStateRemove {
					// log a warning, though. we shouln't be trying to rewrite
					// a state to an earlier state
					log.G(ctx).Warnf(
						"cannot update task %v in desired state %v to an earlier desired state %v",
						latestTask.ID, latestTask.DesiredState, api.TaskStateRemove,
					)
					return nil
				}
				// update desired state to REMOVE
				latestTask.DesiredState = api.TaskStateRemove

				if err := store.UpdateTask(tx, latestTask); err != nil {
					log.G(ctx).WithError(err).Errorf("failed transaction: update task desired state to REMOVE")
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
		log.G(ctx).WithError(err).Errorf("task search transaction failed")
	}
}
