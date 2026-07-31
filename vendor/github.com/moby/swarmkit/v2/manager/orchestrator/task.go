package orchestrator

import (
	"time"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/api/defaults"
	"github.com/moby/swarmkit/v2/identity"
	"github.com/moby/swarmkit/v2/log"
	"github.com/moby/swarmkit/v2/manager/constraint"
	"github.com/moby/swarmkit/v2/protobuf/ptypes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NewTask creates a new task.
func NewTask(cluster *api.Cluster, service *api.Service, slot uint64, nodeID string) *api.Task {
	var logDriver *api.Driver
	if service.GetSpec().GetTask().GetLogDriver() != nil {
		// use the log driver specific to the task, if we have it.
		logDriver = service.GetSpec().GetTask().GetLogDriver()
	} else if cluster != nil {
		// pick up the cluster default, if available.
		logDriver = cluster.GetSpec().GetTaskDefaults().GetLogDriver() // nil is okay here.
	}

	taskID := identity.NewID()
	task := api.Task{
		Id: taskID,
		// The annotations and the spec are copied, not referenced: they are
		// messages (pointers) now, and a single service is used to create many
		// tasks, none of which may observe later changes to the service.
		ServiceAnnotations: service.GetSpec().GetAnnotations().Copy(),
		Spec:               service.GetSpec().GetTask().Copy(),
		SpecVersion:        service.SpecVersion,
		ServiceId:          service.Id,
		Slot:               slot,
		Status: &api.TaskStatus{
			State:     api.TaskState_NEW,
			Timestamp: ptypes.MustTimestampProto(time.Now()),
			Message:   "created",
		},
		Endpoint: &api.Endpoint{
			Spec: service.GetSpec().GetEndpoint().Copy(),
		},
		DesiredState: api.TaskState_RUNNING,
		LogDriver:    logDriver,
	}

	// In global mode we also set the NodeID
	if nodeID != "" {
		task.NodeId = nodeID
	}

	return &task
}

// RestartCondition returns the restart condition to apply to this task.
func RestartCondition(task *api.Task) api.RestartPolicy_RestartCondition {
	restartCondition := defaults.Service.Task.Restart.Condition
	if task.GetSpec().GetRestart() != nil {
		restartCondition = task.GetSpec().GetRestart().GetCondition()
	}
	return restartCondition
}

// IsTaskDirty determines whether a task matches the given service's spec and
// if the given node satisfies the placement constraints.
// Returns false if the spec version didn't change,
// only the task placement constraints changed and the assigned node
// satisfies the new constraints, or the service task spec and the endpoint spec
// didn't change at all.
// Returns true otherwise.
// Note: for non-failed tasks with a container spec runtime that have already
// pulled the required image (i.e., current state is between READY and
// RUNNING inclusively), the value of the `PullOptions` is ignored.
func IsTaskDirty(s *api.Service, t *api.Task, n *api.Node) bool {
	// If the spec version matches, we know the task is not dirty. However,
	// if it does not match, that doesn't mean the task is dirty, since
	// only a portion of the spec is included in the comparison.
	// Versions are messages now, so compare their contents; comparing the
	// pointers would only tell whether they are the same object.
	if t.SpecVersion != nil && s.SpecVersion != nil && s.SpecVersion.EqualVT(t.SpecVersion) {
		return false
	}

	// Make a deep copy of the service task spec, as it is modified below.
	serviceTaskSpec := s.GetSpec().GetTask().Copy()
	if serviceTaskSpec == nil {
		serviceTaskSpec = &api.TaskSpec{}
	}

	// Task is not dirty if the placement constraints alone changed
	// and the node currently assigned can satisfy the changed constraints.
	if IsTaskDirtyPlacementConstraintsOnly(serviceTaskSpec, t) && nodeMatches(s, n) {
		return false
	}

	// For non-failed tasks with a container spec runtime that have already
	// pulled the required image (i.e., current state is between READY and
	// RUNNING inclusively), ignore the value of the `PullOptions` field by
	// setting the copied service to have the same PullOptions value as the
	// task. A difference in only the `PullOptions` field should not cause
	// a running (or ready to run) task to be considered 'dirty' when we
	// handle updates.
	// See https://github.com/docker/swarmkit/issues/971
	currentState := t.Status.GetState()
	// Ignore PullOpts if the task is desired to be in a "runnable" state
	// and its last known current state is between READY and RUNNING in
	// which case we know that the task either successfully pulled its
	// container image or didn't need to.
	ignorePullOpts := t.DesiredState <= api.TaskState_RUNNING &&
		currentState >= api.TaskState_READY &&
		currentState <= api.TaskState_RUNNING
	if ignorePullOpts && serviceTaskSpec.GetContainer() != nil && t.GetSpec().GetContainer() != nil {
		// Modify the service's container spec.
		serviceTaskSpec.GetContainer().PullOptions = t.GetSpec().GetContainer().GetPullOptions()
	}

	// If the task has no spec, treat it as an empty spec for comparison.
	taskSpec := t.GetSpec()
	if taskSpec == nil {
		taskSpec = &api.TaskSpec{}
	}

	return !serviceTaskSpec.EqualVT(taskSpec) ||
		(t.Endpoint != nil && !s.GetSpec().GetEndpoint().EqualVT(t.Endpoint.Spec))
}

// Checks if the current assigned node matches the Placement.Constraints
// specified in the task spec for Updater.newService.
func nodeMatches(s *api.Service, n *api.Node) bool {
	if n == nil {
		return false
	}

	pc := s.GetSpec().GetTask().GetPlacement().GetConstraints()
	constraints, err := constraint.Parse(pc)
	if err != nil {
		log.L.WithFields(map[string]any{
			"error":       err,
			"constraints": pc,
		}).Debug("IsTaskDirty: nodeMatches: failed to parse placement constraints")
	}
	return constraint.NodeMatches(constraints, n)
}

// IsTaskDirtyPlacementConstraintsOnly checks if the Placement field alone
// in the spec has changed.
func IsTaskDirtyPlacementConstraintsOnly(serviceTaskSpec *api.TaskSpec, t *api.Task) bool {
	// If the task has no spec, treat it as an empty spec for comparison.
	taskSpec := t.GetSpec()
	if taskSpec == nil {
		taskSpec = &api.TaskSpec{}
	}

	// Compare the task placement constraints.
	if serviceTaskSpec.GetPlacement().EqualVT(taskSpec.GetPlacement()) {
		return false
	}

	// Compare the rest of the spec with the placement masked out. The spec is
	// taken by pointer, so mask it on a copy to leave the caller's spec alone.
	masked := serviceTaskSpec.Copy()
	if masked == nil {
		masked = &api.TaskSpec{}
	}
	masked.Placement = taskSpec.GetPlacement()
	return masked.EqualVT(taskSpec)
}

// InvalidNode is true if the node is nil, down, or drained
func InvalidNode(n *api.Node) bool {
	return n == nil ||
		n.GetStatus().GetState() == api.NodeStatus_DOWN ||
		n.GetSpec().GetAvailability() == api.NodeSpec_DRAIN
}

func taskTimestamp(t *api.Task) *timestamppb.Timestamp {
	if t.Status.GetAppliedAt() != nil {
		return t.Status.GetAppliedAt()
	}

	return t.Status.GetTimestamp()
}

// TasksByTimestamp sorts tasks by applied timestamp if available, otherwise
// status timestamp.
type TasksByTimestamp []*api.Task

// Len implements the Len method for sorting.
func (t TasksByTimestamp) Len() int {
	return len(t)
}

// Swap implements the Swap method for sorting.
func (t TasksByTimestamp) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

// Less implements the Less method for sorting.
func (t TasksByTimestamp) Less(i, j int) bool {
	iTimestamp := taskTimestamp(t[i])
	jTimestamp := taskTimestamp(t[j])

	if iTimestamp == nil {
		return true
	}
	if jTimestamp == nil {
		return false
	}
	if iTimestamp.Seconds < jTimestamp.Seconds {
		return true
	}
	if iTimestamp.Seconds > jTimestamp.Seconds {
		return false
	}
	return iTimestamp.Nanos < jTimestamp.Nanos
}
