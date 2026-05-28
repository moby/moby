package service

import (
	"path/filepath"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
	"gotest.tools/v3/assert"
)

func TestSetGetMeta(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	db, err := bolt.Open(filepath.Join(tmpDir, "db"), 0o600, &bolt.Options{Timeout: 1 * time.Second})
	assert.NilError(t, err)

	store := &VolumeStore{db: db}
	defer store.Shutdown()

	_, err = store.getMeta("test")
	assert.ErrorContains(t, err, "")

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
