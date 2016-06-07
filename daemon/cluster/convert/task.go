package convert

import (
	types "github.com/docker/engine-api/types/swarm"
	swarmapi "github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/protobuf/ptypes"
)

// TaskFromGRPC converts a grpc Task to a Task.
func TaskFromGRPC(t swarmapi.Task) types.Task {
	containerConfig := t.Spec.Runtime.(*swarmapi.TaskSpec_Container).Container
	containerStatus := t.Status.GetContainer()
	task := types.Task{
		ID:        t.ID,
		ServiceID: t.ServiceID,
		Instance:  int(t.Instance),
		NodeID:    t.NodeID,
		Spec: types.TaskSpec{
			ContainerSpec: containerSpecFromGRPC(containerConfig),
			Resources:     resourcesFromGRPC(t.Spec.Resources),
			RestartPolicy: restartPolicyFromGRPC(t.Spec.Restart),
			Placement:     placementFromGRPC(t.Spec.Placement),
		},
		Status: types.TaskStatus{
			State:   types.TaskState(t.Status.State.String()),
			Message: t.Status.Message,
			Err:     t.Status.Err,
		},
		DesiredState: types.TaskState(t.DesiredState.String()),
	}

	// Meta
	task.Version.Index = t.Meta.Version.Index
	task.CreatedAt, _ = ptypes.Timestamp(t.Meta.CreatedAt)
	task.UpdatedAt, _ = ptypes.Timestamp(t.Meta.UpdatedAt)

	task.Status.Timestamp, _ = ptypes.Timestamp(t.Status.Timestamp)

	if containerStatus != nil {
		task.Status.ContainerStatus.ContainerID = containerStatus.ContainerID
		task.Status.ContainerStatus.PID = int(containerStatus.PID)
		task.Status.ContainerStatus.ExitCode = int(containerStatus.ExitCode)
	}

	// NetworksAttachments
	for _, na := range t.Networks {
		task.NetworksAttachments = append(task.NetworksAttachments, networkAttachementFromGRPC(na))
	}

	return task
}
