package replicated

import (
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/orchestrator"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
)

type slotsByRunningState []orchestrator.Slot

func (is slotsByRunningState) Len() int      { return len(is) }
func (is slotsByRunningState) Swap(i, j int) { is[i], is[j] = is[j], is[i] }

// Less returns true if the first task should be preferred over the second task,
// all other things being equal in terms of node balance.
func (is slotsByRunningState) Less(i, j int) bool {
	iRunning := false
	jRunning := false

	for _, ii := range is[i] {
		if ii.Status.State == api.TaskStateRunning {
			iRunning = true
			break
		}
	}
	for _, ij := range is[j] {
		if ij.Status.State == api.TaskStateRunning {
			jRunning = true
			break
		}
	}

	if iRunning && !jRunning {
		return true
	}

	if !iRunning && jRunning {
		return false
	}

	// Use Slot number as a tie-breaker to prefer to remove tasks in reverse
	// order of Slot number. This would help us avoid unnecessary master
	// migration when scaling down a stateful service because the master
	// task of a stateful service is usually in a low numbered Slot.
	return is[i][0].Slot < is[j][0].Slot
}

type slotWithIndex struct {
	slot orchestrator.Slot

	// index is a counter that counts this task as the nth instance of
	// the service on its node. This is used for sorting the tasks so that
	// when scaling down we leave tasks more evenly balanced.
	index int
}

type slotsByIndex []slotWithIndex

func (is slotsByIndex) Len() int      { return len(is) }
func (is slotsByIndex) Swap(i, j int) { is[i], is[j] = is[j], is[i] }

func (is slotsByIndex) Less(i, j int) bool {
	if is[i].index < 0 && is[j].index >= 0 {
		return false
	}
	if is[j].index < 0 && is[i].index >= 0 {
		return true
	}
	return is[i].index < is[j].index
}

// updatableAndDeadSlots returns two maps of slots. The first contains slots
// that have at least one task with a desired state above NEW and lesser or
// equal to RUNNING, or a task that shouldn't be restarted. The second contains
// all other slots with at least one task.
func (r *Orchestrator) updatableAndDeadSlots(ctx context.Context, service *api.Service) (map[uint64]orchestrator.Slot, map[uint64]orchestrator.Slot, error) {
	var (
		tasks []*api.Task
		err   error
	)
	r.store.View(func(tx store.ReadTx) {
		tasks, err = store.FindTasks(tx, store.ByServiceID(service.ID))
	})
	if err != nil {
		return nil, nil, err
	}

	updatableSlots := make(map[uint64]orchestrator.Slot)
	for _, t := range tasks {
		updatableSlots[t.Slot] = append(updatableSlots[t.Slot], t)
	}

	deadSlots := make(map[uint64]orchestrator.Slot)
	for slotID, slot := range updatableSlots {
		updatable := r.restarts.UpdatableTasksInSlot(ctx, slot, service)
		if len(updatable) != 0 {
			updatableSlots[slotID] = updatable
		} else {
			delete(updatableSlots, slotID)
			deadSlots[slotID] = slot
		}
	}

	return updatableSlots, deadSlots, nil
}

// SlotTuple returns a slot tuple for the replicated service task.
func (r *Orchestrator) SlotTuple(t *api.Task) orchestrator.SlotTuple {
	return orchestrator.SlotTuple{
		ServiceID: t.ServiceID,
		Slot:      t.Slot,
	}
}
