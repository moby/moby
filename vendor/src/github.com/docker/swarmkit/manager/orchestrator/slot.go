package orchestrator

import (
	"github.com/docker/swarmkit/api"
)

// slot is a list of the running tasks occupying a certain slot. Generally this
// will only be one task, but some rolling update situations involve
// temporarily having two running tasks in the same slot. Note that this use of
// "slot" is more generic than the Slot number for replicated services - a node
// is also considered a slot for global services.
type slot []*api.Task

type slotsByRunningState []slot

func (is slotsByRunningState) Len() int      { return len(is) }
func (is slotsByRunningState) Swap(i, j int) { is[i], is[j] = is[j], is[i] }

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

	return iRunning && !jRunning
}

type slotWithIndex struct {
	slot slot

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
