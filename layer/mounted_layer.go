package layer

import (
	"io"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/archive"
)

type mountedLayer struct {
	name         string
	mountID      string
	initID       string
	parent       *roLayer
	path         string
	pathCache    string
	layerStore   *layerStore
	activeMount  int
	activeMountL sync.Mutex
	references   map[RWLayer]*referencedRWLayer
}

func (ml *mountedLayer) cacheParent() string {
	if ml.initID != "" {
		return ml.initID
	}
	if ml.parent != nil {
		return ml.parent.cacheID
	}
	return ""
}

func (ml *mountedLayer) TarStream() (io.ReadCloser, error) {
	archiver, err := ml.layerStore.driver.Diff(ml.mountID, ml.cacheParent())
	if err != nil {
		return nil, err
	}
	return archiver, nil
}

func (ml *mountedLayer) Name() string {
	return ml.name
}

func (ml *mountedLayer) Parent() Layer {
	if ml.parent != nil {
		return ml.parent
	}

	// Return a nil interface instead of an interface wrapping a nil
	// pointer.
	return nil
}

func (ml *mountedLayer) incActiveMount() {
	ml.activeMountL.Lock()
	ml.activeMount++
	ml.activeMountL.Unlock()
}

func (ml *mountedLayer) decActiveMount() {
	ml.activeMountL.Lock()
	ml.activeMount--
	ml.activeMountL.Unlock()
}

func (ml *mountedLayer) Mount(mountLabel string) (string, error) {
	var path string
	var err error

	ml.activeMountL.Lock()
	activeMount := ml.activeMount
	ml.activeMountL.Unlock()

	if activeMount > 0 {
		path = ml.pathCache // load from cache.
		if path == "" {
			path, err = ml.layerStore.store.GetMountPath(ml.name) // load from disk
			if err != nil {
				logrus.Debugf("mountedLayer GetMountPath failed with err %s", err)
				return "", err
			}
		}
		ml.incActiveMount()
		return path, nil
	}

	path, err = ml.layerStore.driver.Get(ml.mountID, mountLabel)
	if err != nil {
		logrus.Debugf("mountedLayer Mount Get failed due to err: %s", err)
		return "", err
	}

	if ml.pathCache == "" {
		err = ml.layerStore.store.SetMountPath(ml.name, path) // save in disk
		if err != nil {
			logrus.Debugf("mountedLayer Mount SetMountPath failed due to err: %s", err)
			return "", err

		}
		ml.pathCache = path // save in cache
	}

	ml.incActiveMount()
	return path, nil
}

func (ml *mountedLayer) Unmount() error {
	ml.activeMountL.Lock()
	activeMount := ml.activeMount
	ml.activeMountL.Unlock()

	if activeMount > 0 {
		ml.decActiveMount()
		return nil
	}
	return ml.layerStore.driver.Put(ml.mountID)
}

func (ml *mountedLayer) Size() (int64, error) {
	return ml.layerStore.driver.DiffSize(ml.mountID, ml.cacheParent())
}

func (ml *mountedLayer) Changes() ([]archive.Change, error) {
	return ml.layerStore.driver.Changes(ml.mountID, ml.cacheParent())
}

func (ml *mountedLayer) Metadata() (map[string]string, error) {
	return ml.layerStore.driver.GetMetadata(ml.mountID)
}

func (ml *mountedLayer) getReference() RWLayer {
	ref := &referencedRWLayer{
		mountedLayer: ml,
	}
	ml.references[ref] = ref

	return ref
}

func (ml *mountedLayer) hasReferences() bool {
	return len(ml.references) > 0
}

func (ml *mountedLayer) incActivityCount(ref RWLayer) error {
	rl, ok := ml.references[ref]
	if !ok {
		return ErrLayerNotRetained
	}

	if err := rl.acquire(); err != nil {
		return err
	}
	return nil
}

func (ml *mountedLayer) deleteReference(ref RWLayer) error {
	rl, ok := ml.references[ref]
	if !ok {
		return ErrLayerNotRetained
	}

	if err := rl.release(); err != nil {
		return err
	}
	delete(ml.references, ref)

	return nil
}

func (ml *mountedLayer) retakeReference(r RWLayer) {
	if ref, ok := r.(*referencedRWLayer); ok {
		ref.activityCount = 0
		ml.references[ref] = ref
	}
}

type referencedRWLayer struct {
	*mountedLayer

	activityL     sync.Mutex
	activityCount int
}

func (rl *referencedRWLayer) acquire() error {
	rl.activityL.Lock()
	defer rl.activityL.Unlock()

	rl.activityCount++
	rl.mountedLayer.incActiveMount()

	return nil
}

func (rl *referencedRWLayer) release() error {
	rl.activityL.Lock()
	defer rl.activityL.Unlock()

	if rl.activityCount > 0 {
		return ErrActiveMount
	}

	rl.activityCount = -1

	return nil
}

func (rl *referencedRWLayer) Mount(mountLabel string) (string, error) {
	rl.activityL.Lock()
	defer rl.activityL.Unlock()

	if rl.activityCount == -1 {
		return "", ErrLayerNotRetained
	}

	m, err := rl.mountedLayer.Mount(mountLabel)
	if err != nil {
		return "", err
	}
	rl.activityCount++
	rl.path = m
	return rl.path, nil

}

// Unmount decrements the activity count and unmounts the underlying layer
// Callers should only call `Unmount` once per call to `Mount`, even on error.
func (rl *referencedRWLayer) Unmount() error {
	rl.activityL.Lock()
	defer rl.activityL.Unlock()

	if rl.activityCount == 0 {
		return ErrNotMounted
	}
	if rl.activityCount == -1 {
		return ErrLayerNotRetained
	}

	rl.activityCount--

	return rl.mountedLayer.Unmount()
}
