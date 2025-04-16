package service // import "github.com/docker/docker/volume/service"

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/containerd/log"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
)

var volumeBucketName = []byte("volumes")

type volumeMetadata struct {
	Name    string
	Driver  string
	Labels  map[string]string
	Options map[string]string
}

func (s *VolumeStore) setMeta(name string, meta volumeMetadata) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return setMeta(tx, name, meta)
	})
}

func setMeta(tx *bolt.Tx, name string, meta volumeMetadata) error {
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	b, err := tx.CreateBucketIfNotExists(volumeBucketName)
	if err != nil {
		return errors.Wrap(err, "error creating volume bucket")
	}
	return errors.Wrap(b.Put([]byte(name), metaJSON), "error setting volume metadata")
}

func (s *VolumeStore) getMeta(name string) (volumeMetadata, error) {
	var meta volumeMetadata
	err := s.db.View(func(tx *bolt.Tx) error {
		return getMeta(tx, name, &meta)
	})
	return meta, err
}

func getMeta(tx *bolt.Tx, name string, meta *volumeMetadata) error {
	b := tx.Bucket(volumeBucketName)
	if b == nil {
		return errdefs.NotFound(errors.New("volume bucket does not exist"))
	}
	val := b.Get([]byte(name))
	if len(val) == 0 {
		return nil
	}
	if err := json.Unmarshal(val, meta); err != nil {
		return errors.Wrap(err, "error unmarshaling volume metadata")
	}
	return nil
}

func (s *VolumeStore) removeMeta(name string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return removeMeta(tx, name)
	})
}

func removeMeta(tx *bolt.Tx, name string) error {
	b := tx.Bucket(volumeBucketName)
	return errors.Wrap(b.Delete([]byte(name)), "error removing volume metadata")
}

// listMeta is used during restore to get the list of volume metadata
// from the on-disk database.
// Any errors that occur are only logged.
func listMeta(tx *bolt.Tx, rootPath string) []volumeMetadata {
	var ls []volumeMetadata
	b := tx.Bucket(volumeBucketName)
	b.ForEach(func(k, v []byte) error {
		if len(v) == 0 {
			//Given that emptying the metadata.db of a volume does not affect the use of the volume,
			//back up the database to facilitate restart the docker daemon.
			defer func() {
				if v := recover(); v != nil {
					// Format the date and time, prevent overwriting
					now := time.Now()
					dateString := now.Format("20060102150405")
					dbPath := filepath.Join(rootPath, "volumes/metadata.db")
					errPath := filepath.Join(rootPath, "volumes/metadata.db.err."+dateString)
					if err := os.Rename(dbPath, errPath); err != nil {
						log.L.Errorf("Backup volume's metadata.db err: %v", err)
					}
					log.L.Errorf("Error while reading volume metadata Bucket is nil, The metadata.db backup: %v", errPath)
					log.L.Errorf("List stack:\n %s \n", string(debug.Stack()))
					fmt.Printf("Error while reading volume metadata Bucket is nil, The metadata.db backup: %v", errPath)
					os.Exit(1)
				}
			}()
			//value is empty, debug record. The actual purpose is to directly panic and exit when the key has an invalid value,
			//thus avoiding the creation of an array with invalid values in an infinite loop
			if b.Bucket(k) == nil {
				log.L.Debugf("The value is empty while reading volume metadata : %v", k)
			}
			// don't try to unmarshal an empty value
			return nil
		}

		var m volumeMetadata
		if err := json.Unmarshal(v, &m); err != nil {
			// Just log the error
			log.G(context.TODO()).Errorf("Error while reading volume metadata for volume %q: %v", string(k), err)
			return nil
		}
		ls = append(ls, m)
		return nil
	})
	return ls
}
