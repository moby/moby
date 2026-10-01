package kvstore_test

import (
	"path/filepath"
	"strings"
	"testing"

	store "github.com/moby/moby/v2/daemon/libnetwork/kvstore"
	"github.com/moby/moby/v2/daemon/libnetwork/kvstore/boltdb"
	"github.com/moby/moby/v2/daemon/libnetwork/kvstore/memstore"
	bolt "go.etcd.io/bbolt"
	berrors "go.etcd.io/bbolt/errors"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestStores checks that the stores accept and reject the same operations,
// with the same errors.
func TestStores(t *testing.T) {
	for _, tc := range []struct {
		name     string
		newStore func(*testing.T) store.Store
	}{
		{name: "boltdb", newStore: func(t *testing.T) store.Store {
			s, err := boltdb.New(filepath.Join(t.TempDir(), "local-kv.db"), "test")
			assert.NilError(t, err)
			return s
		}},
		{name: "memstore", newStore: func(*testing.T) store.Store { return memstore.New() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("Put", func(t *testing.T) {
				s := tc.newStore(t)
				defer s.Close()

				ok, err := s.Exists("a/1")
				assert.Check(t, is.ErrorIs(err, store.ErrKeyNotFound))
				assert.Check(t, !ok)
				_, err = s.List("a/")
				assert.Check(t, is.ErrorIs(err, store.ErrKeyNotFound))

				assert.NilError(t, s.Put("a/2", []byte("two")))
				assert.NilError(t, s.Put("a/1", []byte("one")))
				assert.NilError(t, s.Put("b/1", []byte("other")))
				ok, err = s.Exists("a/1")
				assert.Check(t, err)
				assert.Check(t, ok)

				kvs, err := s.List("a/")
				assert.NilError(t, err)
				assert.Assert(t, is.Len(kvs, 2))
				assert.Check(t, is.Equal(kvs[0].Key, "a/1"))
				assert.Check(t, is.Equal(string(kvs[0].Value), "one"))
				assert.Check(t, is.Equal(kvs[1].Key, "a/2"))
				assert.Check(t, is.Equal(string(kvs[1].Value), "two"))
				assert.Check(t, kvs[0].LastIndex > kvs[1].LastIndex, "indexes don't follow the order of the writes")

				assert.NilError(t, s.Put("c/nil", nil))
				kvs, err = s.List("c/")
				assert.NilError(t, err)
				assert.Assert(t, is.Len(kvs, 1))
				assert.Check(t, kvs[0].Value != nil, "listed a nil value")
				assert.Check(t, is.Len(kvs[0].Value, 0))
			})

			t.Run("AtomicPut", func(t *testing.T) {
				s := tc.newStore(t)
				defer s.Close()

				_, err := s.AtomicPut("k", []byte("v"), &store.KVPair{Key: "k", LastIndex: 1})
				assert.Check(t, is.ErrorIs(err, store.ErrKeyNotFound), "update of a missing key")

				first, err := s.AtomicPut("k", []byte("v1"), nil)
				assert.NilError(t, err)
				_, err = s.AtomicPut("k", []byte("v"), nil)
				assert.Check(t, is.ErrorIs(err, store.ErrKeyExists), "create of an existing key")

				second, err := s.AtomicPut("k", []byte("v2"), first)
				assert.NilError(t, err)
				assert.Check(t, second.LastIndex > first.LastIndex)
				_, err = s.AtomicPut("k", []byte("v"), first)
				assert.Check(t, is.ErrorIs(err, store.ErrKeyModified), "update through a stale pair")

				kvs, err := s.List("k")
				assert.NilError(t, err)
				assert.Assert(t, is.Len(kvs, 1))
				assert.Check(t, is.Equal(string(kvs[0].Value), "v2"))
				assert.Check(t, is.Equal(kvs[0].LastIndex, second.LastIndex))
			})

			t.Run("AtomicDelete", func(t *testing.T) {
				s := tc.newStore(t)
				defer s.Close()

				assert.Check(t, is.ErrorIs(s.AtomicDelete("k", &store.KVPair{Key: "k", LastIndex: 1}), store.ErrKeyNotFound), "delete of a missing key")

				first, err := s.AtomicPut("k", []byte("v1"), nil)
				assert.NilError(t, err)
				second, err := s.AtomicPut("k", []byte("v2"), first)
				assert.NilError(t, err)
				assert.Check(t, is.ErrorIs(s.AtomicDelete("k", nil), store.ErrPreviousNotSpecified))
				assert.Check(t, is.ErrorIs(s.AtomicDelete("k", first), store.ErrKeyModified), "delete through a stale pair")
				assert.NilError(t, s.AtomicDelete("k", second))
				_, err = s.Exists("k")
				assert.Check(t, is.ErrorIs(err, store.ErrKeyNotFound))
				assert.Check(t, is.ErrorIs(s.AtomicDelete("k", second), store.ErrKeyNotFound), "second delete")

				// A key created again gets an index none of its earlier
				// versions had, so a pair from before the delete is stale.
				third, err := s.AtomicPut("k", []byte("v3"), nil)
				assert.NilError(t, err)
				assert.Check(t, third.LastIndex > second.LastIndex)
				_, err = s.AtomicPut("k", []byte("v"), second)
				assert.Check(t, is.ErrorIs(err, store.ErrKeyModified), "update through a pair from before the delete")
				assert.Check(t, is.ErrorIs(s.AtomicDelete("k", second), store.ErrKeyModified), "delete through a pair from before the delete")
			})

			t.Run("InvalidKey", func(t *testing.T) {
				s := tc.newStore(t)
				defer s.Close()

				long := strings.Repeat("k", bolt.MaxKeySize+1)
				assert.Check(t, is.ErrorIs(s.Put("", []byte("v")), berrors.ErrKeyRequired))
				assert.Check(t, is.ErrorIs(s.Put(long, []byte("v")), berrors.ErrKeyTooLarge))
				_, err := s.AtomicPut("", []byte("v"), nil)
				assert.Check(t, is.ErrorIs(err, berrors.ErrKeyRequired))
				_, err = s.AtomicPut(long, []byte("v"), nil)
				assert.Check(t, is.ErrorIs(err, berrors.ErrKeyTooLarge))

				assert.NilError(t, s.Put("k", []byte("v")))
				kvs, err := s.List("k")
				assert.NilError(t, err)
				assert.Check(t, is.Len(kvs, 1), "stored a key it rejected")
			})

			t.Run("Delete", func(t *testing.T) {
				s := tc.newStore(t)
				defer s.Close()

				assert.Check(t, is.ErrorIs(s.Delete("k"), store.ErrKeyNotFound))
				assert.NilError(t, s.Put("k", []byte("v")))
				assert.NilError(t, s.Delete("k"))
				_, err := s.Exists("k")
				assert.Check(t, is.ErrorIs(err, store.ErrKeyNotFound))
			})
		})
	}
}
