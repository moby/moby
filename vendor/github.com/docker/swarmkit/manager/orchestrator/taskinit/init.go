package taskinit

import (
	"time"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/defaults"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/orchestrator/restart"
	"github.com/docker/swarmkit/manager/state/store"
	gogotypes "github.com/gogo/protobuf/types"
	"golang.org/x/net/context"
)

// InitHandler defines orchestrator's action to fix tasks at start.
type InitHandler interface {
	IsRelatedService(service *api.Service) bool
	FixTask(ctx context.Context, batch *store.Batch, t *api.Task)
}

// CheckTasks fixes tasks in the store before orchestrator runs. The previous leader might
// not have finished processing their updates and left them in an inconsistent state.
func CheckTasks(ctx context.Context, s *store.MemoryStore, readTx store.ReadTx, initHandler InitHandler, startSupervisor *restart.Supervisor) error {
	_, err := s.Batch(func(batch *store.Batch) error {
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

			// handle task updates from agent which should have been triggered by task update events
			initHandler.FixTask(ctx, batch, t)

			// desired state ready is a transient state that it should be started.
			// however previous leader may not have started it, retry start here
			if t.DesiredState != api.TaskStateReady || t.Status.State > api.TaskStateRunning {
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
				timestamp, err := gogotypes.TimestampFromProto(t.Status.Timestamp)
				if err == nil {
					restartTime := timestamp.Add(restartDelay)
					calculatedRestartDelay := restartTime.Sub(time.Now())
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
	return err
}
