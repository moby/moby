package store

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/state"
	memdb "github.com/hashicorp/go-memdb"
)

const tableTask = "task"

func init() {
	register(ObjectStoreConfig{
		Name: tableTask,
		Table: &memdb.TableSchema{
			Name: tableTask,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: taskIndexerByID{},
				},
				indexName: {
					Name:         indexName,
					AllowMissing: true,
					Indexer:      taskIndexerByName{},
				},
				indexServiceID: {
					Name:         indexServiceID,
					AllowMissing: true,
					Indexer:      taskIndexerByServiceID{},
				},
				indexNodeID: {
					Name:         indexNodeID,
					AllowMissing: true,
					Indexer:      taskIndexerByNodeID{},
				},
				indexSlot: {
					Name:         indexSlot,
					AllowMissing: true,
					Indexer:      taskIndexerBySlot{},
				},
				indexDesiredState: {
					Name:    indexDesiredState,
					Indexer: taskIndexerByDesiredState{},
				},
			},
		},
		Save: func(tx ReadTx, snapshot *api.StoreSnapshot) error {
			var err error
			snapshot.Tasks, err = FindTasks(tx, All)
			return err
		},
		Restore: func(tx Tx, snapshot *api.StoreSnapshot) error {
			tasks, err := FindTasks(tx, All)
			if err != nil {
				return err
			}
			for _, t := range tasks {
				if err := DeleteTask(tx, t.ID); err != nil {
					return err
				}
			}
			for _, t := range snapshot.Tasks {
				if err := CreateTask(tx, t); err != nil {
					return err
				}
			}
			return nil
		},
		ApplyStoreAction: func(tx Tx, sa *api.StoreAction) error {
			switch v := sa.Target.(type) {
			case *api.StoreAction_Task:
				obj := v.Task
				switch sa.Action {
				case api.StoreActionKindCreate:
					return CreateTask(tx, obj)
				case api.StoreActionKindUpdate:
					return UpdateTask(tx, obj)
				case api.StoreActionKindRemove:
					return DeleteTask(tx, obj.ID)
				}
			}
			return errUnknownStoreAction
		},
		NewStoreAction: func(c state.Event) (api.StoreAction, error) {
			var sa api.StoreAction
			switch v := c.(type) {
			case state.EventCreateTask:
				sa.Action = api.StoreActionKindCreate
				sa.Target = &api.StoreAction_Task{
					Task: v.Task,
				}
			case state.EventUpdateTask:
				sa.Action = api.StoreActionKindUpdate
				sa.Target = &api.StoreAction_Task{
					Task: v.Task,
				}
			case state.EventDeleteTask:
				sa.Action = api.StoreActionKindRemove
				sa.Target = &api.StoreAction_Task{
					Task: v.Task,
				}
			default:
				return api.StoreAction{}, errUnknownStoreAction
			}
			return sa, nil
		},
	})
}

// TaskName returns the task name from Annotations.Name,
// and, in case Annotations.Name is missing, fallback
// to construct the name from othere information.
func TaskName(t *api.Task) string {
	name := t.Annotations.Name
	if name == "" {
		// If Task name is not assigned then calculated name is used like before.
		// This might be removed in the future.
		// We use the following scheme for Task name:
		// Name := <ServiceAnnotations.Name>.<Slot>.<TaskID> (replicated mode)
		//      := <ServiceAnnotations.Name>.<NodeID>.<TaskID> (global mode)
		if t.Slot != 0 {
			name = fmt.Sprintf("%v.%v.%v", t.ServiceAnnotations.Name, t.Slot, t.ID)
		} else {
			name = fmt.Sprintf("%v.%v.%v", t.ServiceAnnotations.Name, t.NodeID, t.ID)
		}
	}
	return name
}

type taskEntry struct {
	*api.Task
}

func (t taskEntry) ID() string {
	return t.Task.ID
}

func (t taskEntry) Meta() api.Meta {
	return t.Task.Meta
}

func (t taskEntry) SetMeta(meta api.Meta) {
	t.Task.Meta = meta
}

func (t taskEntry) Copy() Object {
	return taskEntry{t.Task.Copy()}
}

func (t taskEntry) EventCreate() state.Event {
	return state.EventCreateTask{Task: t.Task}
}

func (t taskEntry) EventUpdate() state.Event {
	return state.EventUpdateTask{Task: t.Task}
}

func (t taskEntry) EventDelete() state.Event {
	return state.EventDeleteTask{Task: t.Task}
}

// CreateTask adds a new task to the store.
// Returns ErrExist if the ID is already taken.
func CreateTask(tx Tx, t *api.Task) error {
	return tx.create(tableTask, taskEntry{t})
}

// UpdateTask updates an existing task in the store.
// Returns ErrNotExist if the node doesn't exist.
func UpdateTask(tx Tx, t *api.Task) error {
	return tx.update(tableTask, taskEntry{t})
}

// DeleteTask removes a task from the store.
// Returns ErrNotExist if the task doesn't exist.
func DeleteTask(tx Tx, id string) error {
	return tx.delete(tableTask, id)
}

// GetTask looks up a task by ID.
// Returns nil if the task doesn't exist.
func GetTask(tx ReadTx, id string) *api.Task {
	t := tx.get(tableTask, id)
	if t == nil {
		return nil
	}
	return t.(taskEntry).Task
}

// FindTasks selects a set of tasks and returns them.
func FindTasks(tx ReadTx, by By) ([]*api.Task, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byName, byNamePrefix, byIDPrefix, byDesiredState, byNode, byService, bySlot:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	taskList := []*api.Task{}
	appendResult := func(o Object) {
		taskList = append(taskList, o.(taskEntry).Task)
	}

	err := tx.find(tableTask, by, checkType, appendResult)
	return taskList, err
}

type taskIndexerByID struct{}

func (ti taskIndexerByID) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ti taskIndexerByID) FromObject(obj interface{}) (bool, []byte, error) {
	t, ok := obj.(taskEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	val := t.Task.ID + "\x00"
	return true, []byte(val), nil
}

func (ti taskIndexerByID) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	return prefixFromArgs(args...)
}

type taskIndexerByName struct{}

func (ti taskIndexerByName) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ti taskIndexerByName) FromObject(obj interface{}) (bool, []byte, error) {
	t, ok := obj.(taskEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	name := TaskName(t.Task)

	// Add the null character as a terminator
	return true, []byte(strings.ToLower(name) + "\x00"), nil
}

func (ti taskIndexerByName) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	return prefixFromArgs(args...)
}

type taskIndexerByServiceID struct{}

func (ti taskIndexerByServiceID) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ti taskIndexerByServiceID) FromObject(obj interface{}) (bool, []byte, error) {
	t, ok := obj.(taskEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	val := t.ServiceID + "\x00"
	return true, []byte(val), nil
}

type taskIndexerByNodeID struct{}

func (ti taskIndexerByNodeID) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ti taskIndexerByNodeID) FromObject(obj interface{}) (bool, []byte, error) {
	t, ok := obj.(taskEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	val := t.NodeID + "\x00"
	return true, []byte(val), nil
}

type taskIndexerBySlot struct{}

func (ti taskIndexerBySlot) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ti taskIndexerBySlot) FromObject(obj interface{}) (bool, []byte, error) {
	t, ok := obj.(taskEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	val := t.ServiceID + "\x00" + strconv.FormatUint(t.Slot, 10) + "\x00"
	return true, []byte(val), nil
}

type taskIndexerByDesiredState struct{}

func (ni taskIndexerByDesiredState) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ni taskIndexerByDesiredState) FromObject(obj interface{}) (bool, []byte, error) {
	n, ok := obj.(taskEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	return true, []byte(strconv.FormatInt(int64(n.DesiredState), 10) + "\x00"), nil
}
