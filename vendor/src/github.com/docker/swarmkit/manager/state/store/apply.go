package store

import (
	"errors"

	"github.com/docker/go-events"
	"github.com/docker/swarmkit/manager/state"
)

// Apply takes an item from the event stream of one Store and applies it to
// a second Store.
func Apply(store *MemoryStore, item events.Event) (err error) {
	return store.Update(func(tx Tx) error {
		switch v := item.(type) {
		case state.EventCreateTask:
			return CreateTask(tx, v.Task)
		case state.EventUpdateTask:
			return UpdateTask(tx, v.Task)
		case state.EventDeleteTask:
			return DeleteTask(tx, v.Task.ID)

		case state.EventCreateService:
			return CreateService(tx, v.Service)
		case state.EventUpdateService:
			return UpdateService(tx, v.Service)
		case state.EventDeleteService:
			return DeleteService(tx, v.Service.ID)

		case state.EventCreateNetwork:
			return CreateNetwork(tx, v.Network)
		case state.EventUpdateNetwork:
			return UpdateNetwork(tx, v.Network)
		case state.EventDeleteNetwork:
			return DeleteNetwork(tx, v.Network.ID)

		case state.EventCreateNode:
			return CreateNode(tx, v.Node)
		case state.EventUpdateNode:
			return UpdateNode(tx, v.Node)
		case state.EventDeleteNode:
			return DeleteNode(tx, v.Node.ID)

		case state.EventCommit:
			return nil
		}
		return errors.New("unrecognized event type")
	})
}
