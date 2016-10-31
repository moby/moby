package orchestrator

import (
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/state/store"
)

// Slot is a list of the running tasks occupying a certain slot. Generally this
// will only be one task, but some rolling update situations involve
// temporarily having two running tasks in the same slot. Note that this use of
// "slot" is more generic than the Slot number for replicated services - a node
// is also considered a slot for global services.
type Slot []*api.Task

// GetRunnableAndDeadSlots returns two maps of slots. The first contains slots
// that have at least one task with a desired state above NEW and lesser or
// equal to RUNNING. The second is for slots that only contain tasks with a
// desired state above RUNNING.
func GetRunnableAndDeadSlots(s *store.MemoryStore, serviceID string) (map[uint64]Slot, map[uint64]Slot, error) {
	var (
		tasks []*api.Task
		err   error
	)
	s.View(func(tx store.ReadTx) {
		tasks, err = store.FindTasks(tx, store.ByServiceID(serviceID))
	})
	if err != nil {
		return nil, nil, err
	}

	runningSlots := make(map[uint64]Slot)
	for _, t := range tasks {
		if t.DesiredState <= api.TaskStateRunning {
			runningSlots[t.Slot] = append(runningSlots[t.Slot], t)
		}
	}

	deadSlots := make(map[uint64]Slot)
	for _, t := range tasks {
		if _, exists := runningSlots[t.Slot]; !exists {
			deadSlots[t.Slot] = append(deadSlots[t.Slot], t)
		}
	}

	return runningSlots, deadSlots, nil
}
