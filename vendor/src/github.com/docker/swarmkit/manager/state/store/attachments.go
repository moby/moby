package store

import (
	"strings"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/state"
	memdb "github.com/hashicorp/go-memdb"
)

const attachmentTable = "attachment"

func init() {
	register(ObjectStoreConfig{
		Name: attachmentTable,
		Table: &memdb.TableSchema{
			Name: attachmentTable,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:    indexID,
					Unique:  true,
					Indexer: attachmentIndexerByID{},
				},
				indexNodeID: {
					Name:         indexNodeID,
					AllowMissing: true,
					Indexer:      attachmentIndexerByNodeID{},
				},
			},
		},
		Save: func(tx ReadTx, snapshot *api.StoreSnapshot) error {
			var err error
			snapshot.Attachments, err = FindAttachments(tx, All)
			return err
		},
		Restore: func(tx Tx, snapshot *api.StoreSnapshot) error {
			attachments, err := FindAttachments(tx, All)
			if err != nil {
				return err
			}
			for _, a := range attachments {
				if err := DeleteAttachment(tx, a.ID); err != nil {
					return err
				}
			}
			for _, a := range snapshot.Attachments {
				if err := CreateAttachment(tx, a); err != nil {
					return err
				}
			}
			return nil
		},
		ApplyStoreAction: func(tx Tx, sa *api.StoreAction) error {
			switch v := sa.Target.(type) {
			case *api.StoreAction_Attachment:
				obj := v.Attachment
				switch sa.Action {
				case api.StoreActionKindCreate:
					return CreateAttachment(tx, obj)
				case api.StoreActionKindUpdate:
					return UpdateAttachment(tx, obj)
				case api.StoreActionKindRemove:
					return DeleteAttachment(tx, obj.ID)
				}
			}
			return errUnknownStoreAction
		},
		NewStoreAction: func(c state.Event) (api.StoreAction, error) {
			var sa api.StoreAction
			switch v := c.(type) {
			case state.EventCreateAttachment:
				sa.Action = api.StoreActionKindCreate
				sa.Target = &api.StoreAction_Attachment{
					Attachment: v.Attachment,
				}
			case state.EventDeleteAttachment:
				sa.Action = api.StoreActionKindRemove
				sa.Target = &api.StoreAction_Attachment{
					Attachment: v.Attachment,
				}
			case state.EventUpdateAttachment:
				sa.Action = api.StoreActionKindUpdate
				sa.Target = &api.StoreAction_Attachment{
					Attachment: v.Attachment,
				}
			default:
				return api.StoreAction{}, errUnknownStoreAction
			}
			return sa, nil
		},
	})
}

type attachmentEntry struct {
	*api.NetworkAttachment
}

func (a attachmentEntry) ID() string {
	return a.NetworkAttachment.ID
}

func (a attachmentEntry) Meta() api.Meta {
	return a.NetworkAttachment.Meta
}

func (a attachmentEntry) SetMeta(meta api.Meta) {
	a.NetworkAttachment.Meta = meta
}

func (a attachmentEntry) Copy() Object {
	return attachmentEntry{a.NetworkAttachment.Copy()}
}

func (a attachmentEntry) EventCreate() state.Event {
	return state.EventCreateAttachment{Attachment: a.NetworkAttachment}
}

func (a attachmentEntry) EventUpdate() state.Event {
	return state.EventUpdateAttachment{Attachment: a.NetworkAttachment}
}

func (a attachmentEntry) EventDelete() state.Event {
	return state.EventDeleteAttachment{Attachment: a.NetworkAttachment}
}

// CreateAttachment adds a new attachment to the store.
// Returns ErrNameConflict if the ID is already taken.
func CreateAttachment(tx Tx, a *api.NetworkAttachment) error {
	// Ensure the name is not already in use.
	if tx.lookup(attachmentTable, indexName, strings.ToLower(a.Config.Annotations.Name)) != nil {
		return ErrNameConflict
	}

	return tx.create(attachmentTable, attachmentEntry{a})
}

// UpdateAttachment updates an existing attachment in the store.
// Returns ErrNotExist if the attachment doesn't exist.
func UpdateAttachment(tx Tx, a *api.NetworkAttachment) error {
	// Ensure the name is either not in use or already used by this same NetworkAttachment.
	if existing := tx.lookup(attachmentTable, indexName, strings.ToLower(a.Config.Annotations.Name)); existing != nil {
		if existing.ID() != a.ID {
			return ErrNameConflict
		}
	}
	return tx.update(attachmentTable, attachmentEntry{a})
}

// DeleteAttachment removes an attachment from the store.
// Returns ErrNotExist if the attachment doesn't exist.
func DeleteAttachment(tx Tx, id string) error {
	return tx.delete(attachmentTable, id)
}

// GetAttachment looks up an attachment by ID.
// Returns nil if the attachment doesn't exist.
func GetAttachment(tx ReadTx, id string) *api.NetworkAttachment {
	a := tx.get(attachmentTable, id)
	if a == nil {
		return nil
	}
	return a.(attachmentEntry).NetworkAttachment
}

// FindAttachments selects a set of attachments and returns them.
func FindAttachments(tx ReadTx, by By) ([]*api.NetworkAttachment, error) {
	checkType := func(by By) error {
		switch by.(type) {
		case byNode, byName, byIDPrefix:
			return nil
		default:
			return ErrInvalidFindBy
		}
	}

	list := []*api.NetworkAttachment{}
	appendResult := func(o Object) {
		list = append(list, o.(attachmentEntry).NetworkAttachment)
	}

	err := tx.find(attachmentTable, by, checkType, appendResult)

	return list, err
}

type attachmentIndexerByID struct{}

func (ai attachmentIndexerByID) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ai attachmentIndexerByID) FromObject(obj interface{}) (bool, []byte, error) {
	a, ok := obj.(attachmentEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	val := a.NetworkAttachment.ID + "\x00"
	return true, []byte(val), nil
}

func (ai attachmentIndexerByID) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	return prefixFromArgs(args...)
}

type attachmentIndexerByNodeID struct{}

func (ti attachmentIndexerByNodeID) FromArgs(args ...interface{}) ([]byte, error) {
	return fromArgs(args...)
}

func (ti attachmentIndexerByNodeID) FromObject(obj interface{}) (bool, []byte, error) {
	t, ok := obj.(attachmentEntry)
	if !ok {
		panic("unexpected type passed to FromObject")
	}

	// Add the null character as a terminator
	val := t.NodeID + "\x00"
	return true, []byte(val), nil
}
