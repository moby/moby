package storeutils

import (
	"testing"

	"github.com/docker/docker/libnetwork/datastore"
	"gotest.tools/v3/assert"
)

// NewTempStore creates a new temporary libnetwork store for testing purposes.
// The store is created in a temporary directory that is cleaned up when the
// test finishes.
func NewTempStore(t *testing.T) *datastore.Store {
	t.Helper()

	ds, err := datastore.New(t.TempDir(), "libnetwork")
	assert.NilError(t, err)

	return ds
}
