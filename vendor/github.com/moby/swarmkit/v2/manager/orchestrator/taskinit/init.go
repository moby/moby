package taskinit

import (
	"context"
	"sort"
	"time"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/api/defaults"
	"github.com/moby/swarmkit/v2/log"
	"github.com/moby/swarmkit/v2/manager/orchestrator"
	"github.com/moby/swarmkit/v2/manager/orchestrator/restart"
	"github.com/moby/swarmkit/v2/manager/state/store"
	gogotypes "github.com/gogo/protobuf/types"
)

// InitHandler defines orchestrator's action to fix tasks at start.
type InitHandler interface {
	IsRelatedService(service *api.Service) bool
	FixTask(ctx context.Context, batch *store.Batch, t *api.Task)
	SlotTuple(t *api.Task) orchestrator.SlotTuple
}

// CheckTasks fixes tasks in the store before orchestrator runs. The previous leader might
// not have finished processing their updates and left them in an inconsistent state.
func CheckTasks(ctx context.Context, s *store.MemoryStore, readTx store.ReadTx, initHandler InitHandler, startSupervisor restart.SupervisorInterface) error {
	instances := make(map[orchestrator.SlotTuple][]*api.Task)
	err := s.Batch(func(batch *store.Batch) error {
		tasks, err := store.FindTasks(readTx, store.All)
		if err != nil {
			return err
		}
		for _, t := range tasks {
			if t.ServiceID == "" {
				continue
			}

			// TODO(aluzzardi): We should NOT retrieve the service here.
			service := store.GetService(readTx, t.ServiceID)
			if service == nil {
				// Service was deleted
				err := batch.Update(func(tx store.Tx) error {
					return store.DeleteTask(tx, t.ID)
				})
				if err != nil {
					log.G(ctx).WithError(err).Error("failed to delete task")
				}
				continue
			}
			if !initHandler.IsRelatedService(service) {
				continue
			}

			tuple := initHandler.SlotTuple(t)
			instances[tuple] = append(instances[tuple], t)

			// handle task updates from agent which should have been triggered by task update events
			initHandler.FixTask(ctx, batch, t)

			// desired state ready is a transient state that it should be started.
			// however previous leader may not have started it, retry start here
			if t.DesiredState != api.TaskStateReady || t.Status.State > api.TaskStateCompleted {
				continue
			}
			restartDelay, _ := gogotypes.DurationFromProto(defaults.Service.Task.Restart.Delay)
			if t.Spec.Restart != nil && t.Spec.Restart.Delay != nil {
				var err error
				restartDelay, err = gogotypes.DurationFromProto(t.Spec.Restart.Delay)
				if err != nil {
					log.G(ctx).WithError(err).Error("invalid restart delay")
					restartDelay, _ = gogotypes.DurationFromProto(defaults.Service.Task.Restart.Delay)
				}
			}
			if restartDelay != 0 {
				var timestamp time.Time
				if t.Status.AppliedAt != nil {
					timestamp, err = gogotypes.TimestampFromProto(t.Status.AppliedAt)
				} else {
					timestamp, err = gogotypes.TimestampFromProto(t.Status.Timestamp)
				}
				if err == nil {
					restartTime := timestamp.Add(restartDelay)
					calculatedRestartDelay := time.Until(restartTime)
					if calculatedRestartDelay < restartDelay {
						restartDelay = calculatedRestartDelay
					}
					if restartDelay > 0 {
						_ = batch.Update(func(tx store.Tx) error {
							t := store.GetTask(tx, t.ID)
							// TODO(aluzzardi): This is shady as well. We should have a more generic condition.
							if t == nil || t.DesiredState != api.TaskStateReady {
								return nil
							}
							startSupervisor.DelayStart(ctx, tx, nil, t.ID, restartDelay, true)
							return nil
						})
						continue
					}
				} else {
					log.G(ctx).WithError(err).Error("invalid status timestamp")
				}
			}

			// Start now
			err := batch.Update(func(tx store.Tx) error {
				return startSupervisor.StartNow(tx, t.ID)
			})
			if err != nil {
				log.G(ctx).WithError(err).WithField("task.id", t.ID).Error("moving task out of delayed state failed")
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	for tuple, instance := range instances {
		// Find the most current spec version. That's the only one
		// we care about for the purpose of reconstructing restart
		// history.
		maxVersion := uint64(0)
		for _, t := range instance {
			if t.SpecVersion != nil && t.SpecVersion.Index > maxVersion {
				maxVersion = t.SpecVersion.Index
			}
		}

		// Create a new slice with just the current spec version tasks.
		var upToDate []*api.Task
		for _, t := range instance {
			if t.SpecVersion != nil && t.SpecVersion.Index == maxVersion {
				upToDate = append(upToDate, t)
			}
		}

		// Sort by creation timestamp
		sort.Sort(tasksByCreationTimestamp(upToDate))

		// All up-to-date tasks in this instance except the first one
		// should be considered restarted.
		if len(upToDate) < 2 {
			continue
		}
		for _, t := range upToDate[1:] {
			startSupervisor.RecordRestartHistory(tuple, t)
		}
	}
	return nil
}

type tasksByCreationTimestamp []*api.Task

func (t tasksByCreationTimestamp) Len() int {
	return len(t)
}
func (t tasksByCreationTimestamp) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}
func (t tasksByCreationTimestamp) Less(i, j int) bool {
	if t[i].Meta.CreatedAt == nil {
		return true
	}
	if t[j].Meta.CreatedAt == nil {
		return false
	}
	if t[i].Meta.CreatedAt.Seconds < t[j].Meta.CreatedAt.Seconds {
		return true
	}
	if t[i].Meta.CreatedAt.Seconds > t[j].Meta.CreatedAt.Seconds {
		return false
	}
	return t[i].Meta.CreatedAt.Nanos < t[j].Meta.CreatedAt.Nanos
}
