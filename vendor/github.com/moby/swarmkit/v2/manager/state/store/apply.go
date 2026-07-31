package store

import (
	"errors"

	"github.com/docker/go-events"
	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/manager/state"
)

// Apply takes an item from the event stream of one Store and applies it to
// a second Store.
func Apply(store *MemoryStore, item events.Event) (err error) {
	return store.Update(func(tx Tx) error {
		switch v := item.(type) {
		case api.EventCreateTask:
			return CreateTask(tx, v.Task)
		case api.EventUpdateTask:
			return UpdateTask(tx, v.Task)
		case api.EventDeleteTask:
			return DeleteTask(tx, v.Task.Id)

		case api.EventCreateService:
			return CreateService(tx, v.Service)
		case api.EventUpdateService:
			return UpdateService(tx, v.Service)
		case api.EventDeleteService:
			return DeleteService(tx, v.Service.Id)

		case api.EventCreateNetwork:
			return CreateNetwork(tx, v.Network)
		case api.EventUpdateNetwork:
			return UpdateNetwork(tx, v.Network)
		case api.EventDeleteNetwork:
			return DeleteNetwork(tx, v.Network.Id)

		case api.EventCreateNode:
			return CreateNode(tx, v.Node)
		case api.EventUpdateNode:
			return UpdateNode(tx, v.Node)
		case api.EventDeleteNode:
			return DeleteNode(tx, v.Node.Id)

		case state.EventCommit:
			return nil
		}
		return errors.New("unrecognized event type")
	})
}
