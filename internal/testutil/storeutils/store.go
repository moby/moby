package storeutils

import (
	"path/filepath"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/datastore"
	"github.com/moby/moby/v2/daemon/libnetwork/kvstore/boltdb"
	"gotest.tools/v3/assert"
)

// NewTempStore creates a new temporary libnetwork store for testing purposes.
// The store is created in a temporary directory that is cleaned up when the
// test finishes.
func NewTempStore(t *testing.T) *datastore.Store {
	t.Helper()

	kv, err := boltdb.New(filepath.Join(t.TempDir(), "local-kv.db"))
	assert.NilError(t, err)
	t.Cleanup(kv.Close)

	return datastore.New(kv)
}
