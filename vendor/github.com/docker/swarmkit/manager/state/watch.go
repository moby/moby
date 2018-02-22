package state

import (
	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/watch"
)

// EventCommit delineates a transaction boundary.
type EventCommit struct {
	Version *api.Version
}

// Matches returns true if this event is a commit event.
func (e EventCommit) Matches(watchEvent events.Event) bool {
	_, ok := watchEvent.(EventCommit)
	return ok
}

// TaskCheckStateGreaterThan is a TaskCheckFunc for checking task state.
func TaskCheckStateGreaterThan(t1, t2 *api.Task) bool {
	return t2.Status.State > t1.Status.State
}

// NodeCheckState is a NodeCheckFunc for matching node state.
func NodeCheckState(n1, n2 *api.Node) bool {
	return n1.Status.State == n2.Status.State
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
