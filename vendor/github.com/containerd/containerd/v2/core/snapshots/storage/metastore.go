/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package storage provides a metadata storage implementation for snapshot
// drivers. Drive implementations are responsible for starting and managing
// transactions using the defined context creator. This storage package uses
// BoltDB for storing metadata. Access to the raw boltdb transaction is not
// provided, but the stored object is provided by the proto subpackage.
package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/log"
	bolt "go.etcd.io/bbolt"
)

// Transactor is used to finalize an active transaction.
type Transactor interface {
	// Commit commits any changes made during the transaction. On error a
	// caller is expected to clean up any resources which would have relied
	// on data mutated as part of this transaction. Only writable
	// transactions can commit, non-writable must call Rollback.
	Commit() error

	// Rollback rolls back any changes made during the transaction. This
	// must be called on all non-writable transactions and aborted writable
	// transaction.
	Rollback() error
}

// Snapshot hold the metadata for an active or view snapshot transaction. The
// ParentIDs hold the snapshot identifiers for the committed snapshots this
// active or view is based on. The ParentIDs are ordered from the highest to the
// lowest base, meaning they should be applied in order from the last index to
// the first index. The first index should always be considered the active
// snapshot's immediate parent.
type Snapshot struct {
	Kind      snapshots.Kind
	ID        string
	ParentIDs []string
}

// Opt allows to customize BoltDB options. Use with care.
type Opt func(*bolt.Options) error

// MetaStore is used to store metadata related to a snapshot driver. The
// MetaStore is intended to store metadata related to name, state and
// parentage. Using the MetaStore is not required to implement a snapshot
// driver but can be used to handle the persistence and transactional
// complexities of a driver implementation.
type MetaStore struct {
	dbfile string

	dbL  sync.Mutex
	db   *bolt.DB
	opts bolt.Options
}

// NewMetaStore returns a snapshot MetaStore for storage of metadata related to
// a snapshot driver backed by a bolt file database. This implementation is
// strongly consistent and does all metadata changes in a transaction to prevent
// against process crashes causing inconsistent metadata state.
func NewMetaStore(dbfile string, opts ...Opt) (*MetaStore, error) {
	store := &MetaStore{
		dbfile: dbfile,
		opts:   *bolt.DefaultOptions,
	}

	for _, f := range opts {
		if err := f(&store.opts); err != nil {
			return nil, err
		}
	}

	return store, nil
}

type transactionKey struct{}

// TransactionContext creates a new transaction context. The writable value
// should be set to true for transactions which are expected to mutate data.
func (ms *MetaStore) TransactionContext(ctx context.Context, writable bool) (context.Context, Transactor, error) {
	ms.dbL.Lock()
	if ms.db == nil {
		db, err := bolt.Open(ms.dbfile, 0600, &ms.opts)
		if err != nil {
			ms.dbL.Unlock()
			return ctx, nil, fmt.Errorf("failed to open database file: %w", err)
		}
		ms.db = db
	}
	ms.dbL.Unlock()

	tx, err := ms.db.Begin(writable)
	if err != nil {
		return ctx, nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	ctx = context.WithValue(ctx, transactionKey{}, tx)

	return ctx, tx, nil
}

// TransactionCallback represents a callback to be invoked while under a metastore transaction.
type TransactionCallback func(ctx context.Context) error

// WithTransaction is a convenience method to run a function `fn` while holding a meta store transaction.
// If the callback `fn` returns an error or the transaction is not writable, the database transaction will be discarded.
func (ms *MetaStore) WithTransaction(ctx context.Context, writable bool, fn TransactionCallback) error {
	ctx, trans, err := ms.TransactionContext(ctx, writable)
	if err != nil {
		return err
	}

	var result []error
	err = fn(ctx)
	if err != nil {
		result = append(result, err)
	}

	// Always rollback if transaction is not writable
	if err != nil || !writable {
		if terr := trans.Rollback(); terr != nil {
			log.G(ctx).WithError(terr).Error("failed to rollback transaction")

			result = append(result, fmt.Errorf("rollback failed: %w", terr))
		}
	} else {
		if terr := trans.Commit(); terr != nil {
			log.G(ctx).WithError(terr).Error("failed to commit transaction")

			result = append(result, fmt.Errorf("commit failed: %w", terr))
		}
	}

	if err := errors.Join(result...); err != nil {
		log.G(ctx).WithError(err).Debug("snapshotter error")
		return err
	}

	return nil
}

// Close closes the metastore and any underlying database connections
func (ms *MetaStore) Close() error {
	ms.dbL.Lock()
	defer ms.dbL.Unlock()
	if ms.db == nil {
		return nil
	}
	return ms.db.Close()
}
