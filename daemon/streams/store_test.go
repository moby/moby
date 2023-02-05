package streams

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/streams"
	"go.etcd.io/bbolt"
	"gotest.tools/v3/assert"
)

func TestStore(t *testing.T) {
	dir := t.TempDir()

	db, err := bbolt.Open(filepath.Join(dir, "db"), 0600, &bbolt.Options{Timeout: 10 * time.Second})
	assert.NilError(t, err)
	defer db.Close()

	s := NewStore(db)
	defer s.Close()

	id := "foo"
	err = s.Create(streams.Stream{ID: id})
	assert.NilError(t, err)

	stream, err := s.Get(id)
	assert.NilError(t, err)
	assert.Equal(t, stream.ID, id)
}
