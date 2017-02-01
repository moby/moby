package convert

import (
	"strings"

	types "github.com/docker/docker/api/types/swarm"
	swarmapi "github.com/docker/swarmkit/api"
	gogotypes "github.com/gogo/protobuf/types"
)

// TaskFromGRPC converts a grpc Task to a Task.
func TaskFromGRPC(t swarmapi.Task) types.Task {
	if t.Spec.GetAttachment() != nil {
		return types.Task{}
	}
	containerConfig := t.Spec.Runtime.(*swarmapi.TaskSpec_Container).Container
	containerStatus := t.Status.GetContainer()
	networks := make([]types.NetworkAttachmentConfig, 0, len(t.Spec.Networks))
	for _, n := range t.Spec.Networks {
		networks = append(networks, types.NetworkAttachmentConfig{Target: n.Target, Aliases: n.Aliases})
	}

	task := types.Task{
		ID: t.ID,
		Annotations: types.Annotations{
			Name:   t.Annotations.Name,
			Labels: t.Annotations.Labels,
		},
		ServiceID: t.ServiceID,
		Slot:      int(t.Slot),
		NodeID:    t.NodeID,
		Spec: types.TaskSpec{
			ContainerSpec: containerSpecFromGRPC(containerConfig),
			Resources:     resourcesFromGRPC(t.Spec.Resources),
			RestartPolicy: restartPolicyFromGRPC(t.Spec.Restart),
			Placement:     placementFromGRPC(t.Spec.Placement),
			LogDriver:     driverFromGRPC(t.Spec.LogDriver),
			Networks:      networks,
		},
		Status: types.TaskStatus{
			State:   types.TaskState(strings.ToLower(t.Status.State.String())),
			Message: t.Status.Message,
			Err:     t.Status.Err,
		},
		DesiredState: types.TaskState(strings.ToLower(t.DesiredState.String())),
	}

	// Meta
	task.Version.Index = t.Meta.Version.Index
	task.CreatedAt, _ = gogotypes.TimestampFromProto(t.Meta.CreatedAt)
	task.UpdatedAt, _ = gogotypes.TimestampFromProto(t.Meta.UpdatedAt)

	task.Status.Timestamp, _ = gogotypes.TimestampFromProto(t.Status.Timestamp)

	if containerStatus != nil {
		task.Status.ContainerStatus.ContainerID = containerStatus.ContainerID
		task.Status.ContainerStatus.PID = int(containerStatus.PID)
		task.Status.ContainerStatus.ExitCode = int(containerStatus.ExitCode)
	}

	// NetworksAttachments
	for _, na := range t.Networks {
		task.NetworksAttachments = append(task.NetworksAttachments, networkAttachementFromGRPC(na))
	}

	if t.Status.PortStatus == nil {
		return task
	}

	for _, p := range t.Status.PortStatus.Ports {
		task.Status.PortStatus.Ports = append(task.Status.PortStatus.Ports, types.PortConfig{
			Name:          p.Name,
			Protocol:      types.PortConfigProtocol(strings.ToLower(swarmapi.PortConfig_Protocol_name[int32(p.Protocol)])),
			PublishMode:   types.PortConfigPublishMode(strings.ToLower(swarmapi.PortConfig_PublishMode_name[int32(p.PublishMode)])),
			TargetPort:    p.TargetPort,
			PublishedPort: p.PublishedPort,
		})
	}

	return task
}
