package layer

import (
	"context"
	"time"

	"github.com/containerd/containerd/filters"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
)

// SnapshotterAdapter exposes moby's layer store as a containerd's Snapshotter so the layer can access legacy blobs
type SnapshotterAdapter struct {
	layerStore layerStore
}

// Stat implements snapshots.Snapshotter#Stat.
func (s *SnapshotterAdapter) Stat(ctx context.Context, key string) (snapshots.Info, error) {
	id := ChainID(key)
	parent, err := s.layerStore.store.GetParent(id)
	if err != nil {
		return snapshots.Info{}, err
	}

	return snapshots.Info{
		Kind:    snapshots.KindActive,
		Name:    "", //FIXME
		Parent:  parent.String(),
		Labels:  nil,
		Created: time.Time{}, //FIXME
		Updated: time.Time{}, //FIXME
	}, nil
}

// Update implements snapshots.Snapshotter#Update.
func (s *SnapshotterAdapter) Update(ctx context.Context, info snapshots.Info, fieldpaths ...string) (snapshots.Info, error) {
}

// Usage implements snapshots.Snapshotter#Usage.
func (s *SnapshotterAdapter) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	id := ChainID(key)
	size, err := s.layerStore.store.GetSize(id)
	if err != nil {
		return snapshots.Usage{}, err
	}
	return snapshots.Usage{
		Inodes: 0, // FIXME doesn't seem Moby has such data
		Size:   size,
	}, nil
}

// Mounts implements snapshots.Snapshotter#Mounts.
func (s *SnapshotterAdapter) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {
	var mounts []mount.Mount
	// TODO lock
	for _, mountedLayer := range s.layerStore.mounts {
		if mountedLayer.mountID == key { // FIXME unclear to me this is the right way to filter mountedLayers by layer ID
			mounts = append(mounts, mount.Mount{
				Type:    "",
				Source:  "",
				Options: nil, // FIXME would need to collect mount data passed to driver.CreateReadWrite
			})
		}
	}
	return mounts, nil
}

// Prepare  implements snapshots.Snapshotter#Prepare.
func (s *SnapshotterAdapter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	_, err := s.layerStore.CreateRWLayer(key, ChainID(parent), nil)
	// FIXME return []mount.Mount - while a slice, and not a single mount in the API?
	return nil, err
}

// View  implements snapshots.Snapshotter#View.
func (s *SnapshotterAdapter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	// FIXME layerStore.CreateROLayer ?
	return nil, nil
}

// Commit  implements snapshots.Snapshotter#Commit.
func (s *SnapshotterAdapter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error {
	layer, err := s.layerStore.Get(ChainID(key))
	if err != nil {
		return err
	}
	parent := layer.Parent().ChainID()
	stream, err := layer.TarStream()
	if err != nil {
		return err
	}
	// FIXME give committed layer a name
	_, err = s.layerStore.Register(stream, parent)
	return err
}

// Remove  implements snapshots.Snapshotter#Remove.
func (s *SnapshotterAdapter) Remove(ctx context.Context, key string) error {
	return s.layerStore.driver.Remove(key)
}

// Walk  implements snapshots.Snapshotter#Walk.
func (s *SnapshotterAdapter) Walk(ctx context.Context, fn snapshots.WalkFunc, fs ...string) error {
	filter, err := filters.ParseAll(fs...)
	if err != nil {
		return err
	}
	// TODO locks
	for _, layer := range s.layerStore.layerMap {
		if !filter.Match(adaptLayer(layer)) {
			continue
		}
		if err := fn(ctx, snapshots.Info{ /* TODO convert layer to Info */ }); err != nil {
			return err
		}
	}
}

// TODO apply same logic as metadaata.adaptSnapshot but using layer
func adaptLayer(layer *roLayer) filters.Adaptor {
	return filters.AdapterFunc(func(fieldpath []string) (string, bool) {
		if len(fieldpath) == 0 {
			return "", false
		}
		switch fieldpath[0] {
		case "kind":
			// TODO
		case "name":
			// TODO
		case "parent":
			// TODO
		case "labels":
			// TODO
		}

		return "", false
	})
}
