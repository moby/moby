package store

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/go-events"
	"github.com/docker/swarmkit/api"
	pb "github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"github.com/docker/swarmkit/watch"
	memdb "github.com/hashicorp/go-memdb"
	"golang.org/x/net/context"
)

const (
	indexID           = "id"
	indexName         = "name"
	indexServiceID    = "serviceid"
	indexNodeID       = "nodeid"
	indexSlot         = "slot"
	indexCN           = "cn"
	indexDesiredState = "desiredstate"
	indexTaskState    = "taskstate"
	indexRole         = "role"
	indexMembership   = "membership"
	indexNetwork      = "network"
	indexSecret       = "secret"

	prefix = "_prefix"

	// MaxChangesPerTransaction is the number of changes after which a new
	// transaction should be started within Batch.
	MaxChangesPerTransaction = 200

	// MaxTransactionBytes is the maximum serialized transaction size.
	MaxTransactionBytes = 1.5 * 1024 * 1024
)

var (
	// ErrExist is returned by create operations if the provided ID is already
	// taken.
	ErrExist = errors.New("object already exists")

	// ErrNotExist is returned by altering operations (update, delete) if the
	// provided ID is not found.
	ErrNotExist = errors.New("object does not exist")

	// ErrNameConflict is returned by create/update if the object name is
	// already in use by another object.
	ErrNameConflict = errors.New("name conflicts with an existing object")

	// ErrInvalidFindBy is returned if an unrecognized type is passed to Find.
	ErrInvalidFindBy = errors.New("invalid find argument type")

	// ErrSequenceConflict is returned when trying to update an object
	// whose sequence information does not match the object in the store's.
	ErrSequenceConflict = errors.New("update out of sequence")

	objectStorers []ObjectStoreConfig
	schema        = &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{},
	}
	errUnknownStoreAction = errors.New("unknown store action")
)

func register(os ObjectStoreConfig) {
	objectStorers = append(objectStorers, os)
	schema.Tables[os.Name] = os.Table
}

// MemoryStore is a concurrency-safe, in-memory implementation of the Store
// interface.
type MemoryStore struct {
	// updateLock must be held during an update transaction.
	updateLock sync.Mutex

	memDB *memdb.MemDB
	queue *watch.Queue

	proposer state.Proposer
}

// NewMemoryStore returns an in-memory store. The argument is an optional
// Proposer which will be used to propagate changes to other members in a
// cluster.
func NewMemoryStore(proposer state.Proposer) *MemoryStore {
	memDB, err := memdb.NewMemDB(schema)
	if err != nil {
		// This shouldn't fail
		panic(err)
	}

	return &MemoryStore{
		memDB:    memDB,
		queue:    watch.NewQueue(),
		proposer: proposer,
	}
}

// Close closes the memory store and frees its associated resources.
func (s *MemoryStore) Close() error {
	return s.queue.Close()
}

func fromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	arg, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("argument must be a string: %#v", args[0])
	}
	// Add the null character as a terminator
	arg += "\x00"
	return []byte(arg), nil
}

func prefixFromArgs(args ...interface{}) ([]byte, error) {
	val, err := fromArgs(args...)
	if err != nil {
		return nil, err
	}

	// Strip the null terminator, the rest is a prefix
	n := len(val)
	if n > 0 {
		return val[:n-1], nil
	}
	return val, nil
}

// ReadTx is a read transaction. Note that transaction does not imply
// any internal batching. It only means that the transaction presents a
// consistent view of the data that cannot be affected by other
// transactions.
type ReadTx interface {
	lookup(table, index, id string) Object
	get(table, id string) Object
	find(table string, by By, checkType func(By) error, appendResult func(Object)) error
}

type readTx struct {
	memDBTx *memdb.Txn
}

// View executes a read transaction.
func (s *MemoryStore) View(cb func(ReadTx)) {
	memDBTx := s.memDB.Txn(false)

	readTx := readTx{
		memDBTx: memDBTx,
	}
	cb(readTx)
	memDBTx.Commit()
}

// Tx is a read/write transaction. Note that transaction does not imply
// any internal batching. The purpose of this transaction is to give the
// user a guarantee that its changes won't be visible to other transactions
// until the transaction is over.
type Tx interface {
	ReadTx
	create(table string, o Object) error
	update(table string, o Object) error
	delete(table, id string) error
}

type tx struct {
	readTx
	curVersion *api.Version
	changelist []state.Event
}

// ApplyStoreActions updates a store based on StoreAction messages.
func (s *MemoryStore) ApplyStoreActions(actions []*api.StoreAction) error {
	s.updateLock.Lock()
	memDBTx := s.memDB.Txn(true)

	tx := tx{
		readTx: readTx{
			memDBTx: memDBTx,
		},
	}

	for _, sa := range actions {
		if err := applyStoreAction(&tx, sa); err != nil {
			memDBTx.Abort()
			s.updateLock.Unlock()
			return err
		}
	}

	memDBTx.Commit()

	for _, c := range tx.changelist {
		s.queue.Publish(c)
	}
	if len(tx.changelist) != 0 {
		s.queue.Publish(state.EventCommit{})
	}
	s.updateLock.Unlock()
	return nil
}

func applyStoreAction(tx Tx, sa *api.StoreAction) error {
	for _, os := range objectStorers {
		err := os.ApplyStoreAction(tx, sa)
		if err != errUnknownStoreAction {
			return err
		}
	}

	return errors.New("unrecognized action type")
}

func (s *MemoryStore) update(proposer state.Proposer, cb func(Tx) error) error {
	s.updateLock.Lock()
	memDBTx := s.memDB.Txn(true)

	var curVersion *api.Version

	if proposer != nil {
		curVersion = proposer.GetVersion()
	}

	var tx tx
	tx.init(memDBTx, curVersion)

	err := cb(&tx)

	if err == nil {
		if proposer == nil {
			memDBTx.Commit()
		} else {
			var sa []*api.StoreAction
			sa, err = tx.changelistStoreActions()

			if err == nil {
				if len(sa) != 0 {
					err = proposer.ProposeValue(context.Background(), sa, func() {
						memDBTx.Commit()
					})
				} else {
					memDBTx.Commit()
				}
			}
		}
	}

	if err == nil {
		for _, c := range tx.changelist {
			s.queue.Publish(c)
		}
		if len(tx.changelist) != 0 {
			s.queue.Publish(state.EventCommit{})
		}
	} else {
		memDBTx.Abort()
	}
	s.updateLock.Unlock()
	return err

}

func (s *MemoryStore) updateLocal(cb func(Tx) error) error {
	return s.update(nil, cb)
}

// Update executes a read/write transaction.
func (s *MemoryStore) Update(cb func(Tx) error) error {
	return s.update(s.proposer, cb)
}

// Batch provides a mechanism to batch updates to a store.
type Batch struct {
	tx    tx
	store *MemoryStore
	// applied counts the times Update has run successfully
	applied int
	// committed is the number of times Update had run successfully as of
	// the time pending changes were committed.
	committed int
	// transactionSizeEstimate is the running count of the size of the
	// current transaction.
	transactionSizeEstimate int
	// changelistLen is the last known length of the transaction's
	// changelist.
	changelistLen int
	err           error
}

// Update adds a single change to a batch. Each call to Update is atomic, but
// different calls to Update may be spread across multiple transactions to
// circumvent transaction size limits.
func (batch *Batch) Update(cb func(Tx) error) error {
	if batch.err != nil {
		return batch.err
	}

	if err := cb(&batch.tx); err != nil {
		return err
	}

	batch.applied++

	for batch.changelistLen < len(batch.tx.changelist) {
		sa, err := newStoreAction(batch.tx.changelist[batch.changelistLen])
		if err != nil {
			return err
		}
		batch.transactionSizeEstimate += sa.Size()
		batch.changelistLen++
	}

	if batch.changelistLen >= MaxChangesPerTransaction || batch.transactionSizeEstimate >= (MaxTransactionBytes*3)/4 {
		if err := batch.commit(); err != nil {
			return err
		}

		// Yield the update lock
		batch.store.updateLock.Unlock()
		runtime.Gosched()
		batch.store.updateLock.Lock()

		batch.newTx()
	}

	return nil
}

func (batch *Batch) newTx() {
	var curVersion *api.Version

	if batch.store.proposer != nil {
		curVersion = batch.store.proposer.GetVersion()
	}

	batch.tx.init(batch.store.memDB.Txn(true), curVersion)
	batch.transactionSizeEstimate = 0
	batch.changelistLen = 0
}

func (batch *Batch) commit() error {
	if batch.store.proposer != nil {
		var sa []*api.StoreAction
		sa, batch.err = batch.tx.changelistStoreActions()

		if batch.err == nil {
			if len(sa) != 0 {
				batch.err = batch.store.proposer.ProposeValue(context.Background(), sa, func() {
					batch.tx.memDBTx.Commit()
				})
			} else {
				batch.tx.memDBTx.Commit()
			}
		}
	} else {
		batch.tx.memDBTx.Commit()
	}

	if batch.err != nil {
		batch.tx.memDBTx.Abort()
		return batch.err
	}

	batch.committed = batch.applied

	for _, c := range batch.tx.changelist {
		batch.store.queue.Publish(c)
	}
	if len(batch.tx.changelist) != 0 {
		batch.store.queue.Publish(state.EventCommit{})
	}

	return nil
}

// Batch performs one or more transactions that allow reads and writes
// It invokes a callback that is passed a Batch object. The callback may
// call batch.Update for each change it wants to make as part of the
// batch. The changes in the batch may be split over multiple
// transactions if necessary to keep transactions below the size limit.
// Batch holds a lock over the state, but will yield this lock every
// it creates a new transaction to allow other writers to proceed.
// Thus, unrelated changes to the state may occur between calls to
// batch.Update.
//
// This method allows the caller to iterate over a data set and apply
// changes in sequence without holding the store write lock for an
// excessive time, or producing a transaction that exceeds the maximum
// size.
//
// Batch returns the number of calls to batch.Update whose changes were
// successfully committed to the store.
func (s *MemoryStore) Batch(cb func(*Batch) error) (int, error) {
	s.updateLock.Lock()

	batch := Batch{
		store: s,
	}
	batch.newTx()

	if err := cb(&batch); err != nil {
		batch.tx.memDBTx.Abort()
		s.updateLock.Unlock()
		return batch.committed, err
	}

	err := batch.commit()
	s.updateLock.Unlock()
	return batch.committed, err
}

func (tx *tx) init(memDBTx *memdb.Txn, curVersion *api.Version) {
	tx.memDBTx = memDBTx
	tx.curVersion = curVersion
	tx.changelist = nil
}

func newStoreAction(c state.Event) (*api.StoreAction, error) {
	for _, os := range objectStorers {
		sa, err := os.NewStoreAction(c)
		if err == nil {
			return &sa, nil
		} else if err != errUnknownStoreAction {
			return nil, err
		}
	}

	return nil, errors.New("unrecognized event type")
}

func (tx tx) changelistStoreActions() ([]*api.StoreAction, error) {
	var actions []*api.StoreAction

	for _, c := range tx.changelist {
		sa, err := newStoreAction(c)
		if err != nil {
			return nil, err
		}
		actions = append(actions, sa)
	}

	return actions, nil
}

// lookup is an internal typed wrapper around memdb.
func (tx readTx) lookup(table, index, id string) Object {
	j, err := tx.memDBTx.First(table, index, id)
	if err != nil {
		return nil
	}
	if j != nil {
		return j.(Object)
	}
	return nil
}

// create adds a new object to the store.
// Returns ErrExist if the ID is already taken.
func (tx *tx) create(table string, o Object) error {
	if tx.lookup(table, indexID, o.ID()) != nil {
		return ErrExist
	}

	copy := o.Copy()
	meta := copy.Meta()
	if err := touchMeta(&meta, tx.curVersion); err != nil {
		return err
	}
	copy.SetMeta(meta)

	err := tx.memDBTx.Insert(table, copy)
	if err == nil {
		tx.changelist = append(tx.changelist, copy.EventCreate())
		o.SetMeta(meta)
	}
	return err
}

// Update updates an existing object in the store.
// Returns ErrNotExist if the object doesn't exist.
func (tx *tx) update(table string, o Object) error {
	oldN := tx.lookup(table, indexID, o.ID())
	if oldN == nil {
		return ErrNotExist
	}

	if tx.curVersion != nil {
		if oldN.(Object).Meta().Version != o.Meta().Version {
			return ErrSequenceConflict
		}
	}

	copy := o.Copy()
	meta := copy.Meta()
	if err := touchMeta(&meta, tx.curVersion); err != nil {
		return err
	}
	copy.SetMeta(meta)

	err := tx.memDBTx.Insert(table, copy)
	if err == nil {
		tx.changelist = append(tx.changelist, copy.EventUpdate())
		o.SetMeta(meta)
	}
	return err
}

// Delete removes an object from the store.
// Returns ErrNotExist if the object doesn't exist.
func (tx *tx) delete(table, id string) error {
	n := tx.lookup(table, indexID, id)
	if n == nil {
		return ErrNotExist
	}

	err := tx.memDBTx.Delete(table, n)
	if err == nil {
		tx.changelist = append(tx.changelist, n.EventDelete())
	}
	return err
}

// Get looks up an object by ID.
// Returns nil if the object doesn't exist.
func (tx readTx) get(table, id string) Object {
	o := tx.lookup(table, indexID, id)
	if o == nil {
		return nil
	}
	return o.Copy()
}

// findIterators returns a slice of iterators. The union of items from these
// iterators provides the result of the query.
func (tx readTx) findIterators(table string, by By, checkType func(By) error) ([]memdb.ResultIterator, error) {
	switch by.(type) {
	case byAll, orCombinator: // generic types
	default: // all other types
		if err := checkType(by); err != nil {
			return nil, err
		}
	}

	switch v := by.(type) {
	case byAll:
		it, err := tx.memDBTx.Get(table, indexID)
		if err != nil {
			return nil, err
		}
		return []memdb.ResultIterator{it}, nil
	case orCombinator:
		var iters []memdb.ResultIterator
		for _, subBy := range v.bys {
			it, err := tx.findIterators(table, subBy, checkType)
			if err != nil {
				return nil, err
			}
			iters = append(iters, it...)
		}
		return iters, nil
	case byName:
		it, err := tx.memDBTx.Get(table, indexName, strings.ToLower(string(v)))
		if err != nil {
			return nil, err
		}
		return []memdb.ResultIterator{it}, nil
	case byCN:
		it, err := tx.memDBTx.Get(table, indexCN, string(v))
		if err != nil {
			return nil, err
		}
		return []memdb.ResultIterator{it}, nil
	case byIDPrefix:
		it, err := tx.memDBTx.Get(table, indexID+prefix, string(v))
		if err != nil {
			return nil, err
		}
		return []memdb.ResultIterator{it}, nil
	case byNamePrefix:
		it, err := tx.memDBTx.Get(table, indexName+prefix, strings.ToLower(string(v)))
		if err != nil {
			return nil, err
		}
		return []memdb.ResultIterator{it}, nil
	case byNode:
		it, err := tx.memDBTx.Get(table, indexNodeID, string(v))
		if err != nil {
			return nil, err
		}
		return []memdb.ResultIterator{it}, nil
	case byService:
		it, err := tx.memDBTx.Get(table, indexServiceID, string(v))
		if err != nil {
			return nil, err
		}
		return []memdb.ResultIterator{it}, nil
	case bySlot:
		it, err := tx.memDBTx.Get(table, indexSlot, v.serviceID+"\x00"+strconv.FormatUint(uint64(v.slot), 10))
		if err != nil {
			return nil, err
		}
		return []memdb.ResultIterator{it}, nil
	case byDesiredState:
		it, err := tx.memDBTx.Get(table, indexDesiredState, strconv.FormatInt(int64(v), 10))
		if err != nil {
			return nil, err
		}
		return []memdb.ResultIterator{it}, nil
	case byTaskState:
		it, err := tx.memDBTx.Get(table, indexTaskState, strconv.FormatInt(int64(v), 10))
		if err != nil {
			return nil, err
		}
		return []memdb.ResultIterator{it}, nil
	case byRole:
		it, err := tx.memDBTx.Get(table, indexRole, strconv.FormatInt(int64(v), 10))
		if err != nil {
			return nil, err
		}
		return []memdb.ResultIterator{it}, nil
	case byMembership:
		it, err := tx.memDBTx.Get(table, indexMembership, strconv.FormatInt(int64(v), 10))
		if err != nil {
			return nil, err
		}
		return []memdb.ResultIterator{it}, nil
	case byReferencedNetworkID:
		it, err := tx.memDBTx.Get(table, indexNetwork, string(v))
		if err != nil {
			return nil, err
		}
		return []memdb.ResultIterator{it}, nil
	case byReferencedSecretID:
		it, err := tx.memDBTx.Get(table, indexSecret, string(v))
		if err != nil {
			return nil, err
		}
		return []memdb.ResultIterator{it}, nil
	default:
		return nil, ErrInvalidFindBy
	}
}

// find selects a set of objects calls a callback for each matching object.
func (tx readTx) find(table string, by By, checkType func(By) error, appendResult func(Object)) error {
	fromResultIterators := func(its ...memdb.ResultIterator) {
		ids := make(map[string]struct{})
		for _, it := range its {
			for {
				obj := it.Next()
				if obj == nil {
					break
				}
				o := obj.(Object)
				id := o.ID()
				if _, exists := ids[id]; !exists {
					appendResult(o.Copy())
					ids[id] = struct{}{}
				}
			}
		}
	}

	iters, err := tx.findIterators(table, by, checkType)
	if err != nil {
		return err
	}

	fromResultIterators(iters...)

	return nil
}

// Save serializes the data in the store.
func (s *MemoryStore) Save(tx ReadTx) (*pb.StoreSnapshot, error) {
	var snapshot pb.StoreSnapshot
	for _, os := range objectStorers {
		if err := os.Save(tx, &snapshot); err != nil {
			return nil, err
		}
	}

	return &snapshot, nil
}

// Restore sets the contents of the store to the serialized data in the
// argument.
func (s *MemoryStore) Restore(snapshot *pb.StoreSnapshot) error {
	return s.updateLocal(func(tx Tx) error {
		for _, os := range objectStorers {
			if err := os.Restore(tx, snapshot); err != nil {
				return err
			}
		}
		return nil
	})
}

// WatchQueue returns the publish/subscribe queue.
func (s *MemoryStore) WatchQueue() *watch.Queue {
	return s.queue
}

// ViewAndWatch calls a callback which can observe the state of this
// MemoryStore. It also returns a channel that will return further events from
// this point so the snapshot can be kept up to date. The watch channel must be
// released with watch.StopWatch when it is no longer needed. The channel is
// guaranteed to get all events after the moment of the snapshot, and only
// those events.
func ViewAndWatch(store *MemoryStore, cb func(ReadTx) error, specifiers ...state.Event) (watch chan events.Event, cancel func(), err error) {
	// Using Update to lock the store and guarantee consistency between
	// the watcher and the the state seen by the callback. snapshotReadTx
	// exposes this Tx as a ReadTx so the callback can't modify it.
	err = store.Update(func(tx Tx) error {
		if err := cb(tx); err != nil {
			return err
		}
		watch, cancel = state.Watch(store.WatchQueue(), specifiers...)
		return nil
	})
	if watch != nil && err != nil {
		cancel()
		cancel = nil
		watch = nil
	}
	return
}

// touchMeta updates an object's timestamps when necessary and bumps the version
// if provided.
func touchMeta(meta *api.Meta, version *api.Version) error {
	// Skip meta update if version is not defined as it means we're applying
	// from raft or restoring from a snapshot.
	if version == nil {
		return nil
	}

	now, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		return err
	}

	meta.Version = *version

	// Updated CreatedAt if not defined
	if meta.CreatedAt == nil {
		meta.CreatedAt = now
	}

	meta.UpdatedAt = now

	return nil
}
