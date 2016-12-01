package store

import (
	"encoding/json"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/boltdb/bolt"
	"github.com/docker/docker/volume"
	"github.com/docker/docker/volume/drivers"
)

// restore is called when a new volume store is created.
// It's primary purpose is to ensure that all drivers' refcounts are set based
// on known volumes after a restart.
// This only attempts to track volumes that are actually stored in the on-disk db.
// It does not probe the available drivers to find anything that may have been added
// out of band.
func (s *VolumeStore) restore() {
	var entries []*dbEntry
	s.db.View(func(tx *bolt.Tx) error {
		entries = listEntries(tx)
		return nil
	})

	chRemove := make(chan []byte, len(entries))
	var wg sync.WaitGroup
	for _, entry := range entries {
		wg.Add(1)
		// this is potentially a very slow operation, so do it in a goroutine
		go func(entry *dbEntry) {
			defer wg.Done()
			var meta volumeMetadata
			if len(entry.Value) != 0 {
				if err := json.Unmarshal(entry.Value, &meta); err != nil {
					logrus.Errorf("Error while reading volume metadata for volume %q: %v", string(entry.Key), err)
					// don't return here, we can try with `getVolume` below
				}
			}

			var v volume.Volume
			var err error
			if meta.Driver != "" {
				v, err = lookupVolume(meta.Driver, string(entry.Key))
				if err != nil && err != errNoSuchVolume {
					logrus.WithError(err).WithField("driver", meta.Driver).WithField("volume", string(entry.Key)).Warn("Error restoring volume")
					return
				}
				if v == nil {
					// doesn't exist in the driver, remove it from the db
					chRemove <- entry.Key
					return
				}
			} else {
				v, err = s.getVolume(string(entry.Key))
				if err != nil {
					if err == errNoSuchVolume {
						chRemove <- entry.Key
					}
					return
				}

				meta.Driver = v.DriverName()
				if err := s.setMeta(v.Name(), meta); err != nil {
					logrus.WithError(err).WithField("driver", meta.Driver).WithField("volume", v.Name()).Warn("Error updating volume metadata on restore")
				}
			}

			// increment driver refcount
			volumedrivers.CreateDriver(meta.Driver)

			// cache the volume
			s.globalLock.Lock()
			s.options[v.Name()] = meta.Options
			s.labels[v.Name()] = meta.Labels
			s.names[v.Name()] = v
			s.globalLock.Unlock()
		}(entry)
	}

	wg.Wait()
	close(chRemove)
	s.db.Update(func(tx *bolt.Tx) error {
		for k := range chRemove {
			if err := removeMeta(tx, string(k)); err != nil {
				logrus.Warnf("Error removing stale entry from volume db: %v", err)
			}
		}
		return nil
	})
}
