package state

import (
	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/watch"
)

// Event is the type used for events passed over watcher channels, and also
// the type used to specify filtering in calls to Watch.
type Event interface {
	// TODO(stevvooe): Consider whether it makes sense to squish both the
	// matcher type and the primary type into the same type. It might be better
	// to build a matcher from an event prototype.

	// matches checks if this item in a watch queue matches the event
	// description.
	matches(events.Event) bool
}

// EventCommit delineates a transaction boundary.
type EventCommit struct{}

func (e EventCommit) matches(watchEvent events.Event) bool {
	_, ok := watchEvent.(EventCommit)
	return ok
}

// TaskCheckFunc is the type of function used to perform filtering checks on
// api.Task structures.
type TaskCheckFunc func(t1, t2 *api.Task) bool

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

// EventCreateTask is the type used to put CreateTask events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventCreateTask struct {
	Task *api.Task
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []TaskCheckFunc
}

func (e EventCreateTask) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventCreateTask)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Task, typedEvent.Task) {
			return false
		}
	}
	return true
}

// EventUpdateTask is the type used to put UpdateTask events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventUpdateTask struct {
	Task *api.Task
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []TaskCheckFunc
}

func (e EventUpdateTask) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventUpdateTask)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Task, typedEvent.Task) {
			return false
		}
	}
	return true
}

// EventDeleteTask is the type used to put DeleteTask events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventDeleteTask struct {
	Task *api.Task
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []TaskCheckFunc
}

func (e EventDeleteTask) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventDeleteTask)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Task, typedEvent.Task) {
			return false
		}
	}
	return true
}

// ServiceCheckFunc is the type of function used to perform filtering checks on
// api.Service structures.
type ServiceCheckFunc func(j1, j2 *api.Service) bool

// ServiceCheckID is a ServiceCheckFunc for matching service IDs.
func ServiceCheckID(j1, j2 *api.Service) bool {
	return j1.ID == j2.ID
}

// EventCreateService is the type used to put CreateService events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventCreateService struct {
	Service *api.Service
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []ServiceCheckFunc
}

func (e EventCreateService) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventCreateService)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Service, typedEvent.Service) {
			return false
		}
	}
	return true
}

// EventUpdateService is the type used to put UpdateService events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventUpdateService struct {
	Service *api.Service
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []ServiceCheckFunc
}

func (e EventUpdateService) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventUpdateService)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Service, typedEvent.Service) {
			return false
		}
	}
	return true
}

// EventDeleteService is the type used to put DeleteService events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventDeleteService struct {
	Service *api.Service
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []ServiceCheckFunc
}

func (e EventDeleteService) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventDeleteService)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Service, typedEvent.Service) {
			return false
		}
	}
	return true
}

// NetworkCheckFunc is the type of function used to perform filtering checks on
// api.Service structures.
type NetworkCheckFunc func(n1, n2 *api.Network) bool

// NetworkCheckID is a NetworkCheckFunc for matching network IDs.
func NetworkCheckID(n1, n2 *api.Network) bool {
	return n1.ID == n2.ID
}

// EventCreateNetwork is the type used to put CreateNetwork events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventCreateNetwork struct {
	Network *api.Network
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []NetworkCheckFunc
}

func (e EventCreateNetwork) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventCreateNetwork)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Network, typedEvent.Network) {
			return false
		}
	}
	return true
}

// EventUpdateNetwork is the type used to put UpdateNetwork events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventUpdateNetwork struct {
	Network *api.Network
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []NetworkCheckFunc
}

func (e EventUpdateNetwork) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventUpdateNetwork)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Network, typedEvent.Network) {
			return false
		}
	}
	return true
}

// EventDeleteNetwork is the type used to put DeleteNetwork events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventDeleteNetwork struct {
	Network *api.Network
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []NetworkCheckFunc
}

func (e EventDeleteNetwork) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventDeleteNetwork)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Network, typedEvent.Network) {
			return false
		}
	}
	return true
}

// NodeCheckFunc is the type of function used to perform filtering checks on
// api.Service structures.
type NodeCheckFunc func(n1, n2 *api.Node) bool

// NodeCheckID is a NodeCheckFunc for matching node IDs.
func NodeCheckID(n1, n2 *api.Node) bool {
	return n1.ID == n2.ID
}

// NodeCheckState is a NodeCheckFunc for matching node state.
func NodeCheckState(n1, n2 *api.Node) bool {
	return n1.Status.State == n2.Status.State
}

// EventCreateNode is the type used to put CreateNode events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventCreateNode struct {
	Node *api.Node
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []NodeCheckFunc
}

func (e EventCreateNode) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventCreateNode)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Node, typedEvent.Node) {
			return false
		}
	}
	return true
}

// EventUpdateNode is the type used to put DeleteNode events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventUpdateNode struct {
	Node *api.Node
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []NodeCheckFunc
}

func (e EventUpdateNode) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventUpdateNode)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Node, typedEvent.Node) {
			return false
		}
	}
	return true
}

// EventDeleteNode is the type used to put DeleteNode events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventDeleteNode struct {
	Node *api.Node
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []NodeCheckFunc
}

func (e EventDeleteNode) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventDeleteNode)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Node, typedEvent.Node) {
			return false
		}
	}
	return true
}

// ClusterCheckFunc is the type of function used to perform filtering checks on
// api.Cluster structures.
type ClusterCheckFunc func(v1, v2 *api.Cluster) bool

// ClusterCheckID is a ClusterCheckFunc for matching volume IDs.
func ClusterCheckID(v1, v2 *api.Cluster) bool {
	return v1.ID == v2.ID
}

// EventCreateCluster is the type used to put CreateCluster events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventCreateCluster struct {
	Cluster *api.Cluster
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []ClusterCheckFunc
}

func (e EventCreateCluster) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventCreateCluster)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Cluster, typedEvent.Cluster) {
			return false
		}
	}
	return true
}

// EventUpdateCluster is the type used to put UpdateCluster events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventUpdateCluster struct {
	Cluster *api.Cluster
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []ClusterCheckFunc
}

func (e EventUpdateCluster) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventUpdateCluster)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Cluster, typedEvent.Cluster) {
			return false
		}
	}
	return true
}

// EventDeleteCluster is the type used to put DeleteCluster events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventDeleteCluster struct {
	Cluster *api.Cluster
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []ClusterCheckFunc
}

func (e EventDeleteCluster) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventDeleteCluster)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Cluster, typedEvent.Cluster) {
			return false
		}
	}
	return true
}

// SecretCheckFunc is the type of function used to perform filtering checks on
// api.Secret structures.
type SecretCheckFunc func(v1, v2 *api.Secret) bool

// SecretCheckID is a SecretCheckFunc for matching volume IDs.
func SecretCheckID(v1, v2 *api.Secret) bool {
	return v1.ID == v2.ID
}

// EventCreateSecret is the type used to put CreateSecret events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventCreateSecret struct {
	Secret *api.Secret
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []SecretCheckFunc
}

func (e EventCreateSecret) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventCreateSecret)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Secret, typedEvent.Secret) {
			return false
		}
	}
	return true
}

// EventUpdateSecret is the type used to put UpdateSecret events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventUpdateSecret struct {
	Secret *api.Secret
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []SecretCheckFunc
}

func (e EventUpdateSecret) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventUpdateSecret)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Secret, typedEvent.Secret) {
			return false
		}
	}
	return true
}

// EventDeleteSecret is the type used to put DeleteSecret events on the
// publish/subscribe queue and filter these events in calls to Watch.
type EventDeleteSecret struct {
	Secret *api.Secret
	// Checks is a list of functions to call to filter events for a watch
	// stream. They are applied with AND logic. They are only applicable for
	// calls to Watch.
	Checks []SecretCheckFunc
}

func (e EventDeleteSecret) matches(watchEvent events.Event) bool {
	typedEvent, ok := watchEvent.(EventDeleteSecret)
	if !ok {
		return false
	}

	for _, check := range e.Checks {
		if !check(e.Secret, typedEvent.Secret) {
			return false
		}
	}
	return true
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
func Watch(queue *watch.Queue, specifiers ...Event) (eventq chan events.Event, cancel func()) {
	if len(specifiers) == 0 {
		return queue.Watch()
	}
	return queue.CallbackWatch(Matcher(specifiers...))
}

// Matcher returns an events.Matcher that matches the specifiers with OR logic.
func Matcher(specifiers ...Event) events.MatcherFunc {
	return events.MatcherFunc(func(event events.Event) bool {
		for _, s := range specifiers {
			if s.matches(event) {
				return true
			}
		}
		return false
	})
}
