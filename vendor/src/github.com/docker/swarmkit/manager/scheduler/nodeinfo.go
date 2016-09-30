package scheduler

import "github.com/docker/swarmkit/api"

// NodeInfo contains a node and some additional metadata.
type NodeInfo struct {
	*api.Node
	Tasks                             map[string]*api.Task
	DesiredRunningTasksCount          int
	DesiredRunningTasksCountByService map[string]int
	AvailableResources                api.Resources
}

func newNodeInfo(n *api.Node, tasks map[string]*api.Task, availableResources api.Resources) NodeInfo {
	nodeInfo := NodeInfo{
		Node:  n,
		Tasks: make(map[string]*api.Task),
		DesiredRunningTasksCountByService: make(map[string]int),
		AvailableResources:                availableResources,
	}

	for _, t := range tasks {
		nodeInfo.addTask(t)
	}
	return nodeInfo
}

// addTask removes a task from nodeInfo if it's tracked there, and returns true
// if nodeInfo was modified.
func (nodeInfo *NodeInfo) removeTask(t *api.Task) bool {
	if nodeInfo.Tasks == nil {
		return false
	}
	oldTask, ok := nodeInfo.Tasks[t.ID]
	if !ok {
		return false
	}

	delete(nodeInfo.Tasks, t.ID)
	if oldTask.DesiredState == api.TaskStateRunning {
		nodeInfo.DesiredRunningTasksCount--
		nodeInfo.DesiredRunningTasksCountByService[t.ServiceID]--
	}

	reservations := taskReservations(t.Spec)
	nodeInfo.AvailableResources.MemoryBytes += reservations.MemoryBytes
	nodeInfo.AvailableResources.NanoCPUs += reservations.NanoCPUs

	return true
}

// addTask adds or updates a task on nodeInfo, and returns true if nodeInfo was
// modified.
func (nodeInfo *NodeInfo) addTask(t *api.Task) bool {
	if nodeInfo.Tasks == nil {
		nodeInfo.Tasks = make(map[string]*api.Task)
	}
	if nodeInfo.DesiredRunningTasksCountByService == nil {
		nodeInfo.DesiredRunningTasksCountByService = make(map[string]int)
	}

	oldTask, ok := nodeInfo.Tasks[t.ID]
	if ok {
		if t.DesiredState == api.TaskStateRunning && oldTask.DesiredState != api.TaskStateRunning {
			nodeInfo.Tasks[t.ID] = t
			nodeInfo.DesiredRunningTasksCount++
			nodeInfo.DesiredRunningTasksCountByService[t.ServiceID]++
			return true
		} else if t.DesiredState != api.TaskStateRunning && oldTask.DesiredState == api.TaskStateRunning {
			nodeInfo.Tasks[t.ID] = t
			nodeInfo.DesiredRunningTasksCount--
			nodeInfo.DesiredRunningTasksCountByService[t.ServiceID]--
			return true
		}
		return false
	}

	nodeInfo.Tasks[t.ID] = t
	reservations := taskReservations(t.Spec)
	nodeInfo.AvailableResources.MemoryBytes -= reservations.MemoryBytes
	nodeInfo.AvailableResources.NanoCPUs -= reservations.NanoCPUs

	if t.DesiredState == api.TaskStateRunning {
		nodeInfo.DesiredRunningTasksCount++
		nodeInfo.DesiredRunningTasksCountByService[t.ServiceID]++
	}

	return true
}

func taskReservations(spec api.TaskSpec) (reservations api.Resources) {
	if spec.Resources != nil && spec.Resources.Reservations != nil {
		reservations = *spec.Resources.Reservations
	}
	return
}
