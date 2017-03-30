package state

import (
	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/watch"
)

// EventCommit delineates a transaction boundary.
type EventCommit struct{}

// Matches returns true if this event is a commit event.
func (e EventCommit) Matches(watchEvent events.Event) bool {
	_, ok := watchEvent.(EventCommit)
	return ok
}

// TaskCheckID is a TaskCheckFunc for matching task IDs.
func TaskCheckID(t1, t2 *api.Task) bool {
	return t1.ID == t2.ID
}

// TaskCheckNodeID is a TaskCheckFunc for matching node IDs.
func TaskCheckNodeID(t1, t2 *api.Task) bool {
	return t1.NodeID == t2.NodeID
}

// TaskCheckServiceID is a TaskCheckFunc for matching service IDs.
func TaskCheckServiceID(t1, t2 *api.Task) bool {
	return t1.ServiceID == t2.ServiceID
}

// TaskCheckStateGreaterThan is a TaskCheckFunc for checking task state.
func TaskCheckStateGreaterThan(t1, t2 *api.Task) bool {
	return t2.Status.State > t1.Status.State
}

// ServiceCheckID is a ServiceCheckFunc for matching service IDs.
func ServiceCheckID(j1, j2 *api.Service) bool {
	return j1.ID == j2.ID
}

// NetworkCheckID is a NetworkCheckFunc for matching network IDs.
func NetworkCheckID(n1, n2 *api.Network) bool {
	return n1.ID == n2.ID
}

// NodeCheckID is a NodeCheckFunc for matching node IDs.
func NodeCheckID(n1, n2 *api.Node) bool {
	return n1.ID == n2.ID
}

// NodeCheckState is a NodeCheckFunc for matching node state.
func NodeCheckState(n1, n2 *api.Node) bool {
	return n1.Status.State == n2.Status.State
}

// ClusterCheckID is a ClusterCheckFunc for matching volume IDs.
func ClusterCheckID(v1, v2 *api.Cluster) bool {
	return v1.ID == v2.ID
}

// SecretCheckID is a SecretCheckFunc for matching secret IDs.
func SecretCheckID(v1, v2 *api.Secret) bool {
	return v1.ID == v2.ID
}

// ResourceCheckID is a ResourceCheckFunc for matching resource IDs.
func ResourceCheckID(v1, v2 *api.Resource) bool {
	return v1.ID == v2.ID
}

// ResourceCheckKind is a ResourceCheckFunc for matching resource kinds.
func ResourceCheckKind(v1, v2 *api.Resource) bool {
	return v1.Kind == v2.Kind
}

// ExtensionCheckID is a ExtensionCheckFunc for matching extension IDs.
func ExtensionCheckID(v1, v2 *api.Extension) bool {
	return v1.ID == v2.ID
}

// ExtensionCheckName is a ExtensionCheckFunc for matching extension names names.
func ExtensionCheckName(v1, v2 *api.Extension) bool {
	return v1.Annotations.Name == v2.Annotations.Name
}

// Watch takes a variable number of events to match against. The subscriber
// will receive events that match any of the arguments passed to Watch.
//
// Examples:
//
// // subscribe to all events
// Watch(q)
//
// // subscribe to all UpdateTask events
// Watch(q, EventUpdateTask{})
//
// // subscribe to all task-related events
// Watch(q, EventUpdateTask{}, EventCreateTask{}, EventDeleteTask{})
//
// // subscribe to UpdateTask for node 123
// Watch(q, EventUpdateTask{Task: &api.Task{NodeID: 123},
//                         Checks: []TaskCheckFunc{TaskCheckNodeID}})
//
// // subscribe to UpdateTask for node 123, as well as CreateTask
// // for node 123 that also has ServiceID set to "abc"
// Watch(q, EventUpdateTask{Task: &api.Task{NodeID: 123},
//                         Checks: []TaskCheckFunc{TaskCheckNodeID}},
//         EventCreateTask{Task: &api.Task{NodeID: 123, ServiceID: "abc"},
//                         Checks: []TaskCheckFunc{TaskCheckNodeID,
//                                                 func(t1, t2 *api.Task) bool {
//                                                         return t1.ServiceID == t2.ServiceID
//                                                 }}})
func Watch(queue *watch.Queue, specifiers ...api.Event) (eventq chan events.Event, cancel func()) {
	if len(specifiers) == 0 {
		return queue.Watch()
	}
	return queue.CallbackWatch(Matcher(specifiers...))
}

// Matcher returns an events.Matcher that Matches the specifiers with OR logic.
func Matcher(specifiers ...api.Event) events.MatcherFunc {
	return events.MatcherFunc(func(event events.Event) bool {
		for _, s := range specifiers {
			if s.Matches(event) {
				return true
			}
		}
		return false
	})
}
