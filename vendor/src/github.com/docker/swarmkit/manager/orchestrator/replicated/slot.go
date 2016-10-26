package replicated

import (
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/orchestrator"
)

type slotsByRunningState []orchestrator.Slot

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
