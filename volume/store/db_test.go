package store

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boltdb/bolt"
	"github.com/stretchr/testify/require"
)

func TestSetGetMeta(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "test-set-get")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	db, err := bolt.Open(filepath.Join(dir, "db"), 0600, &bolt.Options{Timeout: 1 * time.Second})
	require.NoError(t, err)

	store := &VolumeStore{db: db}

	_, err = store.getMeta("test")
	require.Error(t, err)

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket(volumeBucketName)
		return err
	})
	require.NoError(t, err)

	meta, err := store.getMeta("test")
	require.NoError(t, err)
	require.Equal(t, volumeMetadata{}, meta)

	testMeta := volumeMetadata{
		Name:    "test",
		Driver:  "fake",
		Labels:  map[string]string{"a": "1", "b": "2"},
		Options: map[string]string{"foo": "bar"},
	}
	err = store.setMeta("test", testMeta)
	require.NoError(t, err)

	meta, err = store.getMeta("test")
	require.NoError(t, err)
	require.Equal(t, testMeta, meta)
}
