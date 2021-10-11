package service // import "github.com/docker/docker/volume/service"

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSetGetMeta(t *testing.T) {
	t.Parallel()

	dir, err := os.MkdirTemp("", "test-set-get")
	assert.NilError(t, err)
	defer os.RemoveAll(dir)

	db, err := bolt.Open(filepath.Join(dir, "db"), 0600, &bolt.Options{Timeout: 1 * time.Second})
	assert.NilError(t, err)

	store := &VolumeStore{db: db}

	_, err = store.getMeta("test")
	assert.Assert(t, is.ErrorContains(err, ""))

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket(volumeBucketName)
		return err
	})
	assert.NilError(t, err)

	meta, err := store.getMeta("test")
	assert.NilError(t, err)
	assert.DeepEqual(t, volumeMetadata{}, meta)

	testMeta := volumeMetadata{
		Name:    "test",
		Driver:  "fake",
		Labels:  map[string]string{"a": "1", "b": "2"},
		Options: map[string]string{"foo": "bar"},
	}
	err = store.setMeta("test", testMeta)
	assert.NilError(t, err)

	meta, err = store.getMeta("test")
	assert.NilError(t, err)
	assert.DeepEqual(t, testMeta, meta)
}
