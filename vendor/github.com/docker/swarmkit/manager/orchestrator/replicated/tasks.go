package replicated

import (
	"time"

	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/orchestrator"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"golang.org/x/net/context"
)

// This file provides task-level orchestration. It observes changes to task
// and node state and kills/recreates tasks if necessary. This is distinct from
// service-level reconciliation, which observes changes to services and creates
// and/or kills tasks to match the service definition.

func invalidNode(n *api.Node) bool {
	return n == nil ||
		n.Status.State == api.NodeStatus_DOWN ||
		n.Spec.Availability == api.NodeAvailabilityDrain
}

func (r *Orchestrator) initTasks(ctx context.Context, readTx store.ReadTx) error {
	tasks, err := store.FindTasks(readTx, store.All)
	if err != nil {
		return err
	}
	for _, t := range tasks {
		if t.NodeID != "" {
			n := store.GetNode(readTx, t.NodeID)
			if invalidNode(n) && t.Status.State <= api.TaskStateRunning && t.DesiredState <= api.TaskStateRunning {
				r.restartTasks[t.ID] = struct{}{}
			}
		}
	}

	_, err = r.store.Batch(func(batch *store.Batch) error {
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
					log.G(ctx).WithError(err).Error("failed to set task desired state to dead")
				}
				continue
			}
			// TODO(aluzzardi): This is shady. We should have a more generic condition.
			if t.DesiredState != api.TaskStateReady || !orchestrator.IsReplicatedService(service) {
				continue
			}
			restartDelay := orchestrator.DefaultRestartDelay
			if t.Spec.Restart != nil && t.Spec.Restart.Delay != nil {
				var err error
				restartDelay, err = ptypes.Duration(t.Spec.Restart.Delay)
				if err != nil {
					log.G(ctx).WithError(err).Error("invalid restart delay")
					restartDelay = orchestrator.DefaultRestartDelay
				}
			}
			if restartDelay != 0 {
				timestamp, err := ptypes.Timestamp(t.Status.Timestamp)
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
							r.restarts.DelayStart(ctx, tx, nil, t.ID, restartDelay, true)
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
				return r.restarts.StartNow(tx, t.ID)
			})
			if err != nil {
				log.G(ctx).WithError(err).WithField("task.id", t.ID).Error("moving task out of delayed state failed")
			}
		}
		return nil
	})

	return err
}

func (r *Orchestrator) handleTaskEvent(ctx context.Context, event events.Event) {
	switch v := event.(type) {
	case state.EventDeleteNode:
		r.restartTasksByNodeID(ctx, v.Node.ID)
	case state.EventCreateNode:
		r.handleNodeChange(ctx, v.Node)
	case state.EventUpdateNode:
		r.handleNodeChange(ctx, v.Node)
	case state.EventDeleteTask:
		if v.Task.DesiredState <= api.TaskStateRunning {
			service := r.resolveService(ctx, v.Task)
			if !orchestrator.IsReplicatedService(service) {
				return
			}
			r.reconcileServices[service.ID] = service
		}
		r.restarts.Cancel(v.Task.ID)
	case state.EventUpdateTask:
		r.handleTaskChange(ctx, v.Task)
	case state.EventCreateTask:
		r.handleTaskChange(ctx, v.Task)
	}
}

func (r *Orchestrator) tickTasks(ctx context.Context) {
	if len(r.restartTasks) > 0 {
		_, err := r.store.Batch(func(batch *store.Batch) error {
			for taskID := range r.restartTasks {
				err := batch.Update(func(tx store.Tx) error {
					// TODO(aaronl): optimistic update?
					t := store.GetTask(tx, taskID)
					if t != nil {
						if t.DesiredState > api.TaskStateRunning {
							return nil
						}

						service := store.GetService(tx, t.ServiceID)
						if !orchestrator.IsReplicatedService(service) {
							return nil
						}

						// Restart task if applicable
						if err := r.restarts.Restart(ctx, tx, r.cluster, service, *t); err != nil {
							return err
						}
					}
					return nil
				})
				if err != nil {
					log.G(ctx).WithError(err).Errorf("Orchestrator task reaping transaction failed")
				}
			}
			return nil
		})

		if err != nil {
			log.G(ctx).WithError(err).Errorf("orchestrator task removal batch failed")
		}

		r.restartTasks = make(map[string]struct{})
	}
}

func (r *Orchestrator) restartTasksByNodeID(ctx context.Context, nodeID string) {
	var err error
	r.store.View(func(tx store.ReadTx) {
		var tasks []*api.Task
		tasks, err = store.FindTasks(tx, store.ByNodeID(nodeID))
		if err != nil {
			return
		}

		for _, t := range tasks {
			if t.DesiredState > api.TaskStateRunning {
				continue
			}
			service := store.GetService(tx, t.ServiceID)
			if orchestrator.IsReplicatedService(service) {
				r.restartTasks[t.ID] = struct{}{}
			}
		}
	})
	if err != nil {
		log.G(ctx).WithError(err).Errorf("failed to list tasks to remove")
	}
}

func (r *Orchestrator) handleNodeChange(ctx context.Context, n *api.Node) {
	if !invalidNode(n) {
		return
	}

	r.restartTasksByNodeID(ctx, n.ID)
}

func (r *Orchestrator) handleTaskChange(ctx context.Context, t *api.Task) {
	// If we already set the desired state past TaskStateRunning, there is no
	// further action necessary.
	if t.DesiredState > api.TaskStateRunning {
		return
	}

	var (
		n       *api.Node
		service *api.Service
	)
	r.store.View(func(tx store.ReadTx) {
		if t.NodeID != "" {
			n = store.GetNode(tx, t.NodeID)
		}
		if t.ServiceID != "" {
			service = store.GetService(tx, t.ServiceID)
		}
	})

	if !orchestrator.IsReplicatedService(service) {
		return
	}

	if t.Status.State > api.TaskStateRunning ||
		(t.NodeID != "" && invalidNode(n)) {
		r.restartTasks[t.ID] = struct{}{}
	}
}
