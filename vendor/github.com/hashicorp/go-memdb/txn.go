package memdb

import (
	"bytes"
	"fmt"
	"strings"
	"sync/atomic"
	"unsafe"

	iradix "github.com/hashicorp/go-immutable-radix"
)

const (
	id = "id"
)

var (
	// ErrNotFound is returned when the requested item is not found
	ErrNotFound = fmt.Errorf("not found")
)

// tableIndex is a tuple of (Table, Index) used for lookups
type tableIndex struct {
	Table string
	Index string
}

// Txn is a transaction against a MemDB.
// This can be a read or write transaction.
type Txn struct {
	db      *MemDB
	write   bool
	rootTxn *iradix.Txn
	after   []func()

	// changes is used to track the changes performed during the transaction. If
	// it is nil at transaction start then changes are not tracked.
	changes Changes

	modified map[tableIndex]*iradix.Txn
}

// TrackChanges enables change tracking for the transaction. If called at any
// point before commit, subsequent mutations will be recorded and can be
// retrieved using ChangeSet. Once this has been called on a transaction it
// can't be unset. As with other Txn methods it's not safe to call this from a
// different goroutine than the one making mutations or committing the
// transaction.
func (txn *Txn) TrackChanges() {
	if txn.changes == nil {
		txn.changes = make(Changes, 0, 1)
	}
}

// readableIndex returns a transaction usable for reading the given index in a
// table. If the transaction is a write transaction with modifications, a clone of the
// modified index will be returned.
func (txn *Txn) readableIndex(table, index string) *iradix.Txn {
	// Look for existing transaction
	if txn.write && txn.modified != nil {
		key := tableIndex{table, index}
		exist, ok := txn.modified[key]
		if ok {
			return exist.Clone()
		}
	}

	// Create a read transaction
	path := indexPath(table, index)
	raw, _ := txn.rootTxn.Get(path)
	indexTxn := raw.(*iradix.Tree).Txn()
	return indexTxn
}

// writableIndex returns a transaction usable for modifying the
// given index in a table.
func (txn *Txn) writableIndex(table, index string) *iradix.Txn {
	if txn.modified == nil {
		txn.modified = make(map[tableIndex]*iradix.Txn)
	}

	// Look for existing transaction
	key := tableIndex{table, index}
	exist, ok := txn.modified[key]
	if ok {
		return exist
	}

	// Start a new transaction
	path := indexPath(table, index)
	raw, _ := txn.rootTxn.Get(path)
	indexTxn := raw.(*iradix.Tree).Txn()

	// If we are the primary DB, enable mutation tracking. Snapshots should
	// not notify, otherwise we will trigger watches on the primary DB when
	// the writes will not be visible.
	indexTxn.TrackMutate(txn.db.primary)

	// Keep this open for the duration of the txn
	txn.modified[key] = indexTxn
	return indexTxn
}

// Abort is used to cancel this transaction.
// This is a noop for read transactions.
func (txn *Txn) Abort() {
	// Noop for a read transaction
	if !txn.write {
		return
	}

	// Check if already aborted or committed
	if txn.rootTxn == nil {
		return
	}

	// Clear the txn
	txn.rootTxn = nil
	txn.modified = nil
	txn.changes = nil

	// Release the writer lock since this is invalid
	txn.db.writer.Unlock()
}

// Commit is used to finalize this transaction.
// This is a noop for read transactions.
func (txn *Txn) Commit() {
	// Noop for a read transaction
	if !txn.write {
		return
	}

	// Check if already aborted or committed
	if txn.rootTxn == nil {
		return
	}

	// Commit each sub-transaction scoped to (table, index)
	for key, subTxn := range txn.modified {
		path := indexPath(key.Table, key.Index)
		final := subTxn.CommitOnly()
		txn.rootTxn.Insert(path, final)
	}

	// Update the root of the DB
	newRoot := txn.rootTxn.CommitOnly()
	atomic.StorePointer(&txn.db.root, unsafe.Pointer(newRoot))

	// Now issue all of the mutation updates (this is safe to call
	// even if mutation tracking isn't enabled); we do this after
	// the root pointer is swapped so that waking responders will
	// see the new state.
	for _, subTxn := range txn.modified {
		subTxn.Notify()
	}
	txn.rootTxn.Notify()

	// Clear the txn
	txn.rootTxn = nil
	txn.modified = nil

	// Release the writer lock since this is invalid
	txn.db.writer.Unlock()

	// Run the deferred functions, if any
	for i := len(txn.after); i > 0; i-- {
		fn := txn.after[i-1]
		fn()
	}
}

// Insert is used to add or update an object into the given table.
//
// When updating an object, the obj provided should be a copy rather
// than a value updated in-place. Modifying values in-place that are already
// inserted into MemDB is not supported behavior.
func (txn *Txn) Insert(table string, obj interface{}) error {
	if !txn.write {
		return fmt.Errorf("cannot insert in read-only transaction")
	}

	// Get the table schema
	tableSchema, ok := txn.db.schema.Tables[table]
	if !ok {
		return fmt.Errorf("invalid table '%s'", table)
	}

	// Get the primary ID of the object
	idSchema := tableSchema.Indexes[id]
	idIndexer := idSchema.Indexer.(SingleIndexer)
	ok, idVal, err := idIndexer.FromObject(obj)
	if err != nil {
		return fmt.Errorf("failed to build primary index: %v", err)
	}
	if !ok {
		return fmt.Errorf("object missing primary index")
	}

	// Lookup the object by ID first, to see if this is an update
	idTxn := txn.writableIndex(table, id)
	existing, update := idTxn.Get(idVal)

	// On an update, there is an existing object with the given
	// primary ID. We do the update by deleting the current object
	// and inserting the new object.
	for name, indexSchema := range tableSchema.Indexes {
		indexTxn := txn.writableIndex(table, name)

		// Determine the new index value
		var (
			ok   bool
			vals [][]byte
			err  error
		)
		switch indexer := indexSchema.Indexer.(type) {
		case SingleIndexer:
			var val []byte
			ok, val, err = indexer.FromObject(obj)
			vals = [][]byte{val}
		case MultiIndexer:
			ok, vals, err = indexer.FromObject(obj)
		}
		if err != nil {
			return fmt.Errorf("failed to build index '%s': %v", name, err)
		}

		// Handle non-unique index by computing a unique index.
		// This is done by appending the primary key which must
		// be unique anyways.
		if ok && !indexSchema.Unique {
			for i := range vals {
				vals[i] = append(vals[i], idVal...)
			}
		}

		// Handle the update by deleting from the index first
		if update {
			var (
				okExist   bool
				valsExist [][]byte
				err       error
			)
			switch indexer := indexSchema.Indexer.(type) {
			case SingleIndexer:
				var valExist []byte
				okExist, valExist, err = indexer.FromObject(existing)
				valsExist = [][]byte{valExist}
			case MultiIndexer:
				okExist, valsExist, err = indexer.FromObject(existing)
			}
			if err != nil {
				return fmt.Errorf("failed to build index '%s': %v", name, err)
			}
			if okExist {
				for i, valExist := range valsExist {
					// Handle non-unique index by computing a unique index.
					// This is done by appending the primary key which must
					// be unique anyways.
					if !indexSchema.Unique {
						valExist = append(valExist, idVal...)
					}

					// If we are writing to the same index with the same value,
					// we can avoid the delete as the insert will overwrite the
					// value anyways.
					if i >= len(vals) || !bytes.Equal(valExist, vals[i]) {
						indexTxn.Delete(valExist)
					}
				}
			}
		}

		// If there is no index value, either this is an error or an expected
		// case and we can skip updating
		if !ok {
			if indexSchema.AllowMissing {
				continue
			} else {
				return fmt.Errorf("missing value for index '%s'", name)
			}
		}

		// Update the value of the index
		for _, val := range vals {
			indexTxn.Insert(val, obj)
		}
	}
	if txn.changes != nil {
		txn.changes = append(txn.changes, Change{
			Table:      table,
			Before:     existing, // might be nil on a create
			After:      obj,
			primaryKey: idVal,
		})
	}
	return nil
}

// Delete is used to delete a single object from the given table.
// This object must already exist in the table.
func (txn *Txn) Delete(table string, obj interface{}) error {
	if !txn.write {
		return fmt.Errorf("cannot delete in read-only transaction")
	}

	// Get the table schema
	tableSchema, ok := txn.db.schema.Tables[table]
	if !ok {
		return fmt.Errorf("invalid table '%s'", table)
	}

	// Get the primary ID of the object
	idSchema := tableSchema.Indexes[id]
	idIndexer := idSchema.Indexer.(SingleIndexer)
	ok, idVal, err := idIndexer.FromObject(obj)
	if err != nil {
		return fmt.Errorf("failed to build primary index: %v", err)
	}
	if !ok {
		return fmt.Errorf("object missing primary index")
	}

	// Lookup the object by ID first, check fi we should continue
	idTxn := txn.writableIndex(table, id)
	existing, ok := idTxn.Get(idVal)
	if !ok {
		return ErrNotFound
	}

	// Remove the object from all the indexes
	for name, indexSchema := range tableSchema.Indexes {
		indexTxn := txn.writableIndex(table, name)

		// Handle the update by deleting from the index first
		var (
			ok   bool
			vals [][]byte
			err  error
		)
		switch indexer := indexSchema.Indexer.(type) {
		case SingleIndexer:
			var val []byte
			ok, val, err = indexer.FromObject(existing)
			vals = [][]byte{val}
		case MultiIndexer:
			ok, vals, err = indexer.FromObject(existing)
		}
		if err != nil {
			return fmt.Errorf("failed to build index '%s': %v", name, err)
		}
		if ok {
			// Handle non-unique index by computing a unique index.
			// This is done by appending the primary key which must
			// be unique anyways.
			for _, val := range vals {
				if !indexSchema.Unique {
					val = append(val, idVal...)
				}
				indexTxn.Delete(val)
			}
		}
	}
	if txn.changes != nil {
		txn.changes = append(txn.changes, Change{
			Table:      table,
			Before:     existing,
			After:      nil, // Now nil indicates deletion
			primaryKey: idVal,
		})
	}
	return nil
}

// DeletePrefix is used to delete an entire subtree based on a prefix.
// The given index must be a prefix index, and will be used to perform a scan and enumerate the set of objects to delete.
// These will be removed from all other indexes, and then a special prefix operation will delete the objects from the given index in an efficient subtree delete operation.
// This is useful when you have a very large number of objects indexed by the given index, along with a much smaller number of entries in the other indexes for those objects.
func (txn *Txn) DeletePrefix(table string, prefix_index string, prefix string) (bool, error) {
	if !txn.write {
		return false, fmt.Errorf("cannot delete in read-only transaction")
	}

	if !strings.HasSuffix(prefix_index, "_prefix") {
		return false, fmt.Errorf("Index name for DeletePrefix must be a prefix index, Got %v ", prefix_index)
	}

	deletePrefixIndex := strings.TrimSuffix(prefix_index, "_prefix")

	// Get an iterator over all of the keys with the given prefix.
	entries, err := txn.Get(table, prefix_index, prefix)
	if err != nil {
		return false, fmt.Errorf("failed kvs lookup: %s", err)
	}
	// Get the table schema
	tableSchema, ok := txn.db.schema.Tables[table]
	if !ok {
		return false, fmt.Errorf("invalid table '%s'", table)
	}

	foundAny := false
	for entry := entries.Next(); entry != nil; entry = entries.Next() {
		if !foundAny {
			foundAny = true
		}
		// Get the primary ID of the object
		idSchema := tableSchema.Indexes[id]
		idIndexer := idSchema.Indexer.(SingleIndexer)
		ok, idVal, err := idIndexer.FromObject(entry)
		if err != nil {
			return false, fmt.Errorf("failed to build primary index: %v", err)
		}
		if !ok {
			return false, fmt.Errorf("object missing primary index")
		}
		if txn.changes != nil {
			// Record the deletion
			idTxn := txn.writableIndex(table, id)
			existing, ok := idTxn.Get(idVal)
			if ok {
				txn.changes = append(txn.changes, Change{
					Table:      table,
					Before:     existing,
					After:      nil, // Now nil indicates deletion
					primaryKey: idVal,
				})
			}
		}
		// Remove the object from all the indexes except the given prefix index
		for name, indexSchema := range tableSchema.Indexes {
			if name == deletePrefixIndex {
				continue
			}
			indexTxn := txn.writableIndex(table, name)

			// Handle the update by deleting from the index first
			var (
				ok   bool
				vals [][]byte
				err  error
			)
			switch indexer := indexSchema.Indexer.(type) {
			case SingleIndexer:
				var val []byte
				ok, val, err = indexer.FromObject(entry)
				vals = [][]byte{val}
			case MultiIndexer:
				ok, vals, err = indexer.FromObject(entry)
			}
			if err != nil {
				return false, fmt.Errorf("failed to build index '%s': %v", name, err)
			}

			if ok {
				// Handle non-unique index by computing a unique index.
				// This is done by appending the primary key which must
				// be unique anyways.
				for _, val := range vals {
					if !indexSchema.Unique {
						val = append(val, idVal...)
					}
					indexTxn.Delete(val)
				}
			}
		}

	}
	if foundAny {
		indexTxn := txn.writableIndex(table, deletePrefixIndex)
		ok = indexTxn.DeletePrefix([]byte(prefix))
		if !ok {
			panic(fmt.Errorf("prefix %v matched some entries but DeletePrefix did not delete any ", prefix))
		}
		return true, nil
	}
	return false, nil
}

// DeleteAll is used to delete all the objects in a given table
// matching the constraints on the index
func (txn *Txn) DeleteAll(table, index string, args ...interface{}) (int, error) {
	if !txn.write {
		return 0, fmt.Errorf("cannot delete in read-only transaction")
	}

	// Get all the objects
	iter, err := txn.Get(table, index, args...)
	if err != nil {
		return 0, err
	}

	// Put them into a slice so there are no safety concerns while actually
	// performing the deletes
	var objs []interface{}
	for {
		obj := iter.Next()
		if obj == nil {
			break
		}

		objs = append(objs, obj)
	}

	// Do the deletes
	num := 0
	for _, obj := range objs {
		if err := txn.Delete(table, obj); err != nil {
			return num, err
		}
		num++
	}
	return num, nil
}

// FirstWatch is used to return the first matching object for
// the given constraints on the index along with the watch channel
func (txn *Txn) FirstWatch(table, index string, args ...interface{}) (<-chan struct{}, interface{}, error) {
	// Get the index value
	indexSchema, val, err := txn.getIndexValue(table, index, args...)
	if err != nil {
		return nil, nil, err
	}

	// Get the index itself
	indexTxn := txn.readableIndex(table, indexSchema.Name)

	// Do an exact lookup
	if indexSchema.Unique && val != nil && indexSchema.Name == index {
		watch, obj, ok := indexTxn.GetWatch(val)
		if !ok {
			return watch, nil, nil
		}
		return watch, obj, nil
	}

	// Handle non-unique index by using an iterator and getting the first value
	iter := indexTxn.Root().Iterator()
	watch := iter.SeekPrefixWatch(val)
	_, value, _ := iter.Next()
	return watch, value, nil
}

// LastWatch is used to return the last matching object for
// the given constraints on the index along with the watch channel
func (txn *Txn) LastWatch(table, index string, args ...interface{}) (<-chan struct{}, interface{}, error) {
	// Get the index value
	indexSchema, val, err := txn.getIndexValue(table, index, args...)
	if err != nil {
		return nil, nil, err
	}

	// Get the index itself
	indexTxn := txn.readableIndex(table, indexSchema.Name)

	// Do an exact lookup
	if indexSchema.Unique && val != nil && indexSchema.Name == index {
		watch, obj, ok := indexTxn.GetWatch(val)
		if !ok {
			return watch, nil, nil
		}
		return watch, obj, nil
	}

	// Handle non-unique index by using an iterator and getting the last value
	iter := indexTxn.Root().ReverseIterator()
	watch := iter.SeekPrefixWatch(val)
	_, value, _ := iter.Previous()
	return watch, value, nil
}

// First is used to return the first matching object for
// the given constraints on the index
func (txn *Txn) First(table, index string, args ...interface{}) (interface{}, error) {
	_, val, err := txn.FirstWatch(table, index, args...)
	return val, err
}

// Last is used to return the last matching object for
// the given constraints on the index
func (txn *Txn) Last(table, index string, args ...interface{}) (interface{}, error) {
	_, val, err := txn.LastWatch(table, index, args...)
	return val, err
}

// LongestPrefix is used to fetch the longest prefix match for the given
// constraints on the index. Note that this will not work with the memdb
// StringFieldIndex because it adds null terminators which prevent the
// algorithm from correctly finding a match (it will get to right before the
// null and fail to find a leaf node). This should only be used where the prefix
// given is capable of matching indexed entries directly, which typically only
// applies to a custom indexer. See the unit test for an example.
func (txn *Txn) LongestPrefix(table, index string, args ...interface{}) (interface{}, error) {
	// Enforce that this only works on prefix indexes.
	if !strings.HasSuffix(index, "_prefix") {
		return nil, fmt.Errorf("must use '%s_prefix' on index", index)
	}

	// Get the index value.
	indexSchema, val, err := txn.getIndexValue(table, index, args...)
	if err != nil {
		return nil, err
	}

	// This algorithm only makes sense against a unique index, otherwise the
	// index keys will have the IDs appended to them.
	if !indexSchema.Unique {
		return nil, fmt.Errorf("index '%s' is not unique", index)
	}

	// Find the longest prefix match with the given index.
	indexTxn := txn.readableIndex(table, indexSchema.Name)
	if _, value, ok := indexTxn.Root().LongestPrefix(val); ok {
		return value, nil
	}
	return nil, nil
}

// getIndexValue is used to get the IndexSchema and the value
// used to scan the index given the parameters. This handles prefix based
// scans when the index has the "_prefix" suffix. The index must support
// prefix iteration.
func (txn *Txn) getIndexValue(table, index string, args ...interface{}) (*IndexSchema, []byte, error) {
	// Get the table schema
	tableSchema, ok := txn.db.schema.Tables[table]
	if !ok {
		return nil, nil, fmt.Errorf("invalid table '%s'", table)
	}

	// Check for a prefix scan
	prefixScan := false
	if strings.HasSuffix(index, "_prefix") {
		index = strings.TrimSuffix(index, "_prefix")
		prefixScan = true
	}

	// Get the index schema
	indexSchema, ok := tableSchema.Indexes[index]
	if !ok {
		return nil, nil, fmt.Errorf("invalid index '%s'", index)
	}

	// Hot-path for when there are no arguments
	if len(args) == 0 {
		return indexSchema, nil, nil
	}

	// Special case the prefix scanning
	if prefixScan {
		prefixIndexer, ok := indexSchema.Indexer.(PrefixIndexer)
		if !ok {
			return indexSchema, nil,
				fmt.Errorf("index '%s' does not support prefix scanning", index)
		}

		val, err := prefixIndexer.PrefixFromArgs(args...)
		if err != nil {
			return indexSchema, nil, fmt.Errorf("index error: %v", err)
		}
		return indexSchema, val, err
	}

	// Get the exact match index
	val, err := indexSchema.Indexer.FromArgs(args...)
	if err != nil {
		return indexSchema, nil, fmt.Errorf("index error: %v", err)
	}
	return indexSchema, val, err
}

// ResultIterator is used to iterate over a list of results from a query on a table.
//
// When a ResultIterator is created from a write transaction, the results from
// Next will reflect a snapshot of the table at the time the ResultIterator is
// created.
// This means that calling Insert or Delete on a transaction while iterating is
// allowed, but the changes made by Insert or Delete will not be observed in the
// results returned from subsequent calls to Next. For example if an item is deleted
// from the index used by the iterator it will still be returned by Next. If an
// item is inserted into the index used by the iterator, it will not be returned
// by Next. However, an iterator created after a call to Insert or Delete will
// reflect the modifications.
//
// When a ResultIterator is created from a write transaction, and there are already
// modifications to the index used by the iterator, the modification cache of the
// index will be invalidated. This may result in some additional allocations if
// the same node in the index is modified again.
type ResultIterator interface {
	WatchCh() <-chan struct{}
	// Next returns the next result from the iterator. If there are no more results
	// nil is returned.
	Next() interface{}
}

// Get is used to construct a ResultIterator over all the rows that match the
// given constraints of an index. The index values must match exactly (this
// is not a range-based or prefix-based lookup) by default.
//
// Prefix lookups: if the named index implements PrefixIndexer, you may perform
// prefix-based lookups by appending "_prefix" to the index name. In this
// scenario, the index values given in args are treated as prefix lookups. For
// example, a StringFieldIndex will match any string with the given value
// as a prefix: "mem" matches "memdb".
//
// See the documentation for ResultIterator to understand the behaviour of the
// returned ResultIterator.
func (txn *Txn) Get(table, index string, args ...interface{}) (ResultIterator, error) {
	indexIter, val, err := txn.getIndexIterator(table, index, args...)
	if err != nil {
		return nil, err
	}

	// Seek the iterator to the appropriate sub-set
	watchCh := indexIter.SeekPrefixWatch(val)

	// Create an iterator
	iter := &radixIterator{
		iter:    indexIter,
		watchCh: watchCh,
	}
	return iter, nil
}

// GetReverse is used to construct a Reverse ResultIterator over all the
// rows that match the given constraints of an index.
// The returned ResultIterator's Next() will return the next Previous value.
//
// See the documentation on Get for details on arguments.
// See the documentation for ResultIterator to understand the behaviour of the
// returned ResultIterator.
func (txn *Txn) GetReverse(table, index string, args ...interface{}) (ResultIterator, error) {
	indexIter, val, err := txn.getIndexIteratorReverse(table, index, args...)
	if err != nil {
		return nil, err
	}

	// Seek the iterator to the appropriate sub-set
	watchCh := indexIter.SeekPrefixWatch(val)

	// Create an iterator
	iter := &radixReverseIterator{
		iter:    indexIter,
		watchCh: watchCh,
	}
	return iter, nil
}

// LowerBound is used to construct a ResultIterator over all the the range of
// rows that have an index value greater than or equal to the provide args.
// Calling this then iterating until the rows are larger than required allows
// range scans within an index. It is not possible to watch the resulting
// iterator since the radix tree doesn't efficiently allow watching on lower
// bound changes. The WatchCh returned will be nill and so will block forever.
//
// See the documentation for ResultIterator to understand the behaviour of the
// returned ResultIterator.
func (txn *Txn) LowerBound(table, index string, args ...interface{}) (ResultIterator, error) {
	indexIter, val, err := txn.getIndexIterator(table, index, args...)
	if err != nil {
		return nil, err
	}

	// Seek the iterator to the appropriate sub-set
	indexIter.SeekLowerBound(val)

	// Create an iterator
	iter := &radixIterator{
		iter: indexIter,
	}
	return iter, nil
}

// ReverseLowerBound is used to construct a Reverse ResultIterator over all the
// the range of rows that have an index value less than or equal to the
// provide args.  Calling this then iterating until the rows are lower than
// required allows range scans within an index. It is not possible to watch the
// resulting iterator since the radix tree doesn't efficiently allow watching
// on lower bound changes. The WatchCh returned will be nill and so will block
// forever.
//
// See the documentation for ResultIterator to understand the behaviour of the
// returned ResultIterator.
func (txn *Txn) ReverseLowerBound(table, index string, args ...interface{}) (ResultIterator, error) {
	indexIter, val, err := txn.getIndexIteratorReverse(table, index, args...)
	if err != nil {
		return nil, err
	}

	// Seek the iterator to the appropriate sub-set
	indexIter.SeekReverseLowerBound(val)

	// Create an iterator
	iter := &radixReverseIterator{
		iter: indexIter,
	}
	return iter, nil
}

// objectID is a tuple of table name and the raw internal id byte slice
// converted to a string. It's only converted to a string to make it comparable
// so this struct can be used as a map index.
type objectID struct {
	Table    string
	IndexVal string
}

// mutInfo stores metadata about mutations to allow collapsing multiple
// mutations to the same object into one.
type mutInfo struct {
	firstBefore interface{}
	lastIdx     int
}

// Changes returns the set of object changes that have been made in the
// transaction so far. If change tracking is not enabled it wil always return
// nil. It can be called before or after Commit. If it is before Commit it will
// return all changes made so far which may not be the same as the final
// Changes. After abort it will always return nil. As with other Txn methods
// it's not safe to call this from a different goroutine than the one making
// mutations or committing the transaction. Mutations will appear in the order
// they were performed in the transaction but multiple operations to the same
// object will be collapsed so only the effective overall change to that object
// is present. If transaction operations are dependent (e.g. copy object X to Y
// then delete X) this might mean the set of mutations is incomplete to verify
// history, but it is complete in that the net effect is preserved (Y got a new
// value, X got removed).
func (txn *Txn) Changes() Changes {
	if txn.changes == nil {
		return nil
	}

	// De-duplicate mutations by key so all take effect at the point of the last
	// write but we keep the mutations in order.
	dups := make(map[objectID]mutInfo)
	for i, m := range txn.changes {
		oid := objectID{
			Table:    m.Table,
			IndexVal: string(m.primaryKey),
		}
		// Store the latest mutation index for each key value
		mi, ok := dups[oid]
		if !ok {
			// First entry for key, store the before value
			mi.firstBefore = m.Before
		}
		mi.lastIdx = i
		dups[oid] = mi
	}
	if len(dups) == len(txn.changes) {
		// No duplicates found, fast path return it as is
		return txn.changes
	}

	// Need to remove the duplicates
	cs := make(Changes, 0, len(dups))
	for i, m := range txn.changes {
		oid := objectID{
			Table:    m.Table,
			IndexVal: string(m.primaryKey),
		}
		mi := dups[oid]
		if mi.lastIdx == i {
			// This was the latest value for this key copy it with the before value in
			// case it's different. Note that m is not a pointer so we are not
			// modifying the txn.changeSet here - it's already a copy.
			m.Before = mi.firstBefore

			// Edge case - if the object was inserted and then eventually deleted in
			// the same transaction, then the net affect on that key is a no-op. Don't
			// emit a mutation with nil for before and after as it's meaningless and
			// might violate expectations and cause a panic in code that assumes at
			// least one must be set.
			if m.Before == nil && m.After == nil {
				continue
			}
			cs = append(cs, m)
		}
	}
	// Store the de-duped version in case this is called again
	txn.changes = cs
	return cs
}

func (txn *Txn) getIndexIterator(table, index string, args ...interface{}) (*iradix.Iterator, []byte, error) {
	// Get the index value to scan
	indexSchema, val, err := txn.getIndexValue(table, index, args...)
	if err != nil {
		return nil, nil, err
	}

	// Get the index itself
	indexTxn := txn.readableIndex(table, indexSchema.Name)
	indexRoot := indexTxn.Root()

	// Get an iterator over the index
	indexIter := indexRoot.Iterator()
	return indexIter, val, nil
}

func (txn *Txn) getIndexIteratorReverse(table, index string, args ...interface{}) (*iradix.ReverseIterator, []byte, error) {
	// Get the index value to scan
	indexSchema, val, err := txn.getIndexValue(table, index, args...)
	if err != nil {
		return nil, nil, err
	}

	// Get the index itself
	indexTxn := txn.readableIndex(table, indexSchema.Name)
	indexRoot := indexTxn.Root()

	// Get an interator over the index
	indexIter := indexRoot.ReverseIterator()
	return indexIter, val, nil
}

// Defer is used to push a new arbitrary function onto a stack which
// gets called when a transaction is committed and finished. Deferred
// functions are called in LIFO order, and only invoked at the end of
// write transactions.
func (txn *Txn) Defer(fn func()) {
	txn.after = append(txn.after, fn)
}

// radixIterator is used to wrap an underlying iradix iterator.
// This is much more efficient than a sliceIterator as we are not
// materializing the entire view.
type radixIterator struct {
	iter    *iradix.Iterator
	watchCh <-chan struct{}
}

func (r *radixIterator) WatchCh() <-chan struct{} {
	return r.watchCh
}

func (r *radixIterator) Next() interface{} {
	_, value, ok := r.iter.Next()
	if !ok {
		return nil
	}
	return value
}

type radixReverseIterator struct {
	iter    *iradix.ReverseIterator
	watchCh <-chan struct{}
}

func (r *radixReverseIterator) Next() interface{} {
	_, value, ok := r.iter.Previous()
	if !ok {
		return nil
	}
	return value
}

func (r *radixReverseIterator) WatchCh() <-chan struct{} {
	return r.watchCh
}

// Snapshot creates a snapshot of the current state of the transaction.
// Returns a new read-only transaction or nil if the transaction is already
// aborted or committed.
func (txn *Txn) Snapshot() *Txn {
	if txn.rootTxn == nil {
		return nil
	}

	snapshot := &Txn{
		db:      txn.db,
		rootTxn: txn.rootTxn.Clone(),
	}

	// Commit sub-transactions into the snapshot
	for key, subTxn := range txn.modified {
		path := indexPath(key.Table, key.Index)
		final := subTxn.CommitOnly()
		snapshot.rootTxn.Insert(path, final)
	}

	return snapshot
}
