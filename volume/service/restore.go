package service // import "github.com/docker/docker/volume/service"

import (
	"context"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/docker/docker/volume"
	"github.com/sirupsen/logrus"
)

// restore is called when a new volume store is created.
// It's primary purpose is to ensure that all drivers' refcounts are set based
// on known volumes after a restart.
// This only attempts to track volumes that are actually stored in the on-disk db.
// It does not probe the available drivers to find anything that may have been added
// out of band.
func (s *VolumeStore) restore() {
	var ls []volumeMetadata
	s.db.View(func(tx *bolt.Tx) error {
		ls = listMeta(tx)
		return nil
	})
	ctx := context.Background()

	chRemove := make(chan *volumeMetadata, len(ls))
	var wg sync.WaitGroup
	for _, meta := range ls {
		wg.Add(1)
		// this is potentially a very slow operation, so do it in a goroutine
		go func(meta volumeMetadata) {
			defer wg.Done()

			var v volume.Volume
			var err error
			if meta.Driver != "" {
				v, err = lookupVolume(ctx, s.drivers, meta.Driver, meta.Name)
				if err != nil && err != errNoSuchVolume {
					logrus.WithError(err).WithField("driver", meta.Driver).WithField("volume", meta.Name).Warn("Error restoring volume")
					return
				}
				if v == nil {
					// doesn't exist in the driver, remove it from the db
					chRemove <- &meta
					return
				}
			} else {
				v, err = s.getVolume(ctx, meta.Name, meta.Driver)
				if err != nil {
					if err == errNoSuchVolume {
						chRemove <- &meta
					}
					return
				}

				meta.Driver = v.DriverName()
				if err := s.setMeta(v.Name(), meta); err != nil {
					logrus.WithError(err).WithField("driver", meta.Driver).WithField("volume", v.Name()).Warn("Error updating volume metadata on restore")
				}
			}

			// increment driver refcount
			s.drivers.CreateDriver(meta.Driver)

			// cache the volume
			s.globalLock.Lock()
			s.options[v.Name()] = meta.Options
			s.labels[v.Name()] = meta.Labels
			s.names[v.Name()] = v
			s.refs[v.Name()] = make(map[string]struct{})
			s.globalLock.Unlock()
		}(meta)
	}

	wg.Wait()
	close(chRemove)
	s.db.Update(func(tx *bolt.Tx) error {
		for meta := range chRemove {
			if err := removeMeta(tx, meta.Name); err != nil {
				logrus.WithField("volume", meta.Name).Warnf("Error removing stale entry from volume db: %v", err)
			}
		}
		return nil
	})
}
