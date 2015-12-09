package layer

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"runtime"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/stringid"
	"github.com/vbatts/tar-split/tar/asm"
	"github.com/vbatts/tar-split/tar/storage"
)

// maxLayerDepth represents the maximum number of
// layers which can be chained together. 125 was
// chosen to account for the 127 max in some
// graphdrivers plus the 2 additional layers
// used to create a rwlayer.
const maxLayerDepth = 125

type layerStore struct {
	store  MetadataStore
	driver graphdriver.Driver

	layerMap map[ChainID]*roLayer
	layerL   sync.Mutex

	mounts map[string]*mountedLayer
	mountL sync.Mutex
}

// NewStore creates a new Store instance using
// the provided metadata store and graph driver.
// The metadata store will be used to restore
// the Store.
func NewStore(store MetadataStore, driver graphdriver.Driver) (Store, error) {
	ls := &layerStore{
		store:    store,
		driver:   driver,
		layerMap: map[ChainID]*roLayer{},
		mounts:   map[string]*mountedLayer{},
	}

	ids, mounts, err := store.List()
	if err != nil {
		return nil, err
	}

	for _, id := range ids {
		l, err := ls.loadLayer(id)
		if err != nil {
			logrus.Debugf("Failed to load layer %s: %s", id, err)
		}
		if l.parent != nil {
			l.parent.referenceCount++
		}
	}

	for _, mount := range mounts {
		if err := ls.loadMount(mount); err != nil {
			logrus.Debugf("Failed to load mount %s: %s", mount, err)
		}
	}

	return ls, nil
}

func (ls *layerStore) loadLayer(layer ChainID) (*roLayer, error) {
	cl, ok := ls.layerMap[layer]
	if ok {
		return cl, nil
	}

	diff, err := ls.store.GetDiffID(layer)
	if err != nil {
		return nil, err
	}

	size, err := ls.store.GetSize(layer)
	if err != nil {
		return nil, err
	}

	cacheID, err := ls.store.GetCacheID(layer)
	if err != nil {
		return nil, err
	}

	parent, err := ls.store.GetParent(layer)
	if err != nil {
		return nil, err
	}

	cl = &roLayer{
		chainID:    layer,
		diffID:     diff,
		size:       size,
		cacheID:    cacheID,
		layerStore: ls,
		references: map[Layer]struct{}{},
	}

	if parent != "" {
		p, err := ls.loadLayer(parent)
		if err != nil {
			return nil, err
		}
		cl.parent = p
	}

	ls.layerMap[cl.chainID] = cl

	return cl, nil
}

func (ls *layerStore) loadMount(mount string) error {
	if _, ok := ls.mounts[mount]; ok {
		return nil
	}

	mountID, err := ls.store.GetMountID(mount)
	if err != nil {
		return err
	}

	initID, err := ls.store.GetInitID(mount)
	if err != nil {
		return err
	}

	parent, err := ls.store.GetMountParent(mount)
	if err != nil {
		return err
	}

	ml := &mountedLayer{
		name:       mount,
		mountID:    mountID,
		initID:     initID,
		layerStore: ls,
	}

	if parent != "" {
		p, err := ls.loadLayer(parent)
		if err != nil {
			return err
		}
		ml.parent = p

		p.referenceCount++
	}

	ls.mounts[ml.name] = ml

	return nil
}

func (ls *layerStore) applyTar(tx MetadataTransaction, ts io.Reader, parent string, layer *roLayer) error {
	digester := digest.Canonical.New()
	tr := io.TeeReader(ts, digester.Hash())

	tsw, err := tx.TarSplitWriter()
	if err != nil {
		return err
	}
	metaPacker := storage.NewJSONPacker(tsw)
	defer tsw.Close()

	// we're passing nil here for the file putter, because the ApplyDiff will
	// handle the extraction of the archive
	rdr, err := asm.NewInputTarStream(tr, metaPacker, nil)
	if err != nil {
		return err
	}

	applySize, err := ls.driver.ApplyDiff(layer.cacheID, parent, archive.Reader(rdr))
	if err != nil {
		return err
	}

	// Discard trailing data but ensure metadata is picked up to reconstruct stream
	io.Copy(ioutil.Discard, rdr) // ignore error as reader may be closed

	layer.size = applySize
	layer.diffID = DiffID(digester.Digest())

	logrus.Debugf("Applied tar %s to %s, size: %d", layer.diffID, layer.cacheID, applySize)

	return nil
}

func (ls *layerStore) Register(ts io.Reader, parent ChainID) (Layer, error) {
	// err is used to hold the error which will always trigger
	// cleanup of creates sources but may not be an error returned
	// to the caller (already exists).
	var err error
	var pid string
	var p *roLayer
	if string(parent) != "" {
		p = ls.get(parent)
		if p == nil {
			return nil, ErrLayerDoesNotExist
		}
		pid = p.cacheID
		// Release parent chain if error
		defer func() {
			if err != nil {
				ls.layerL.Lock()
				ls.releaseLayer(p)
				ls.layerL.Unlock()
			}
		}()
		if p.depth() >= maxLayerDepth {
			err = ErrMaxDepthExceeded
			return nil, err
		}
	}

	// Create new roLayer
	layer := &roLayer{
		parent:         p,
		cacheID:        stringid.GenerateRandomID(),
		referenceCount: 1,
		layerStore:     ls,
		references:     map[Layer]struct{}{},
	}

	if err = ls.driver.Create(layer.cacheID, pid, ""); err != nil {
		return nil, err
	}

	tx, err := ls.store.StartTransaction()
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			logrus.Debugf("Cleaning up layer %s: %v", layer.cacheID, err)
			if err := ls.driver.Remove(layer.cacheID); err != nil {
				logrus.Errorf("Error cleaning up cache layer %s: %v", layer.cacheID, err)
			}
			if err := tx.Cancel(); err != nil {
				logrus.Errorf("Error canceling metadata transaction %q: %s", tx.String(), err)
			}
		}
	}()

	if err = ls.applyTar(tx, ts, pid, layer); err != nil {
		return nil, err
	}

	if layer.parent == nil {
		layer.chainID = ChainID(layer.diffID)
	} else {
		layer.chainID = createChainIDFromParent(layer.parent.chainID, layer.diffID)
	}

	if err = storeLayer(tx, layer); err != nil {
		return nil, err
	}

	ls.layerL.Lock()
	defer ls.layerL.Unlock()

	if existingLayer := ls.getWithoutLock(layer.chainID); existingLayer != nil {
		// Set error for cleanup, but do not return the error
		err = errors.New("layer already exists")
		return existingLayer.getReference(), nil
	}

	if err = tx.Commit(layer.chainID); err != nil {
		return nil, err
	}

	ls.layerMap[layer.chainID] = layer

	return layer.getReference(), nil
}

func (ls *layerStore) getWithoutLock(layer ChainID) *roLayer {
	l, ok := ls.layerMap[layer]
	if !ok {
		return nil
	}

	l.referenceCount++

	return l
}

func (ls *layerStore) get(l ChainID) *roLayer {
	ls.layerL.Lock()
	defer ls.layerL.Unlock()
	return ls.getWithoutLock(l)
}

func (ls *layerStore) Get(l ChainID) (Layer, error) {
	layer := ls.get(l)
	if layer == nil {
		return nil, ErrLayerDoesNotExist
	}

	return layer.getReference(), nil
}

func (ls *layerStore) deleteLayer(layer *roLayer, metadata *Metadata) error {
	err := ls.driver.Remove(layer.cacheID)
	if err != nil {
		return err
	}

	err = ls.store.Remove(layer.chainID)
	if err != nil {
		return err
	}
	metadata.DiffID = layer.diffID
	metadata.ChainID = layer.chainID
	metadata.Size, err = layer.Size()
	if err != nil {
		return err
	}
	metadata.DiffSize = layer.size

	return nil
}

func (ls *layerStore) releaseLayer(l *roLayer) ([]Metadata, error) {
	depth := 0
	removed := []Metadata{}
	for {
		if l.referenceCount == 0 {
			panic("layer not retained")
		}
		l.referenceCount--
		if l.referenceCount != 0 {
			return removed, nil
		}

		if len(removed) == 0 && depth > 0 {
			panic("cannot remove layer with child")
		}
		if l.hasReferences() {
			panic("cannot delete referenced layer")
		}
		var metadata Metadata
		if err := ls.deleteLayer(l, &metadata); err != nil {
			return nil, err
		}

		delete(ls.layerMap, l.chainID)
		removed = append(removed, metadata)

		if l.parent == nil {
			return removed, nil
		}

		depth++
		l = l.parent
	}
}

func (ls *layerStore) Release(l Layer) ([]Metadata, error) {
	ls.layerL.Lock()
	defer ls.layerL.Unlock()
	layer, ok := ls.layerMap[l.ChainID()]
	if !ok {
		return []Metadata{}, nil
	}
	if !layer.hasReference(l) {
		return nil, ErrLayerNotRetained
	}

	layer.deleteReference(l)

	return ls.releaseLayer(layer)
}

func (ls *layerStore) mount(m *mountedLayer, mountLabel string) error {
	dir, err := ls.driver.Get(m.mountID, mountLabel)
	if err != nil {
		return err
	}
	m.path = dir
	m.activityCount++

	return nil
}

func (ls *layerStore) saveMount(mount *mountedLayer) error {
	if err := ls.store.SetMountID(mount.name, mount.mountID); err != nil {
		return err
	}

	if mount.initID != "" {
		if err := ls.store.SetInitID(mount.name, mount.initID); err != nil {
			return err
		}
	}

	if mount.parent != nil {
		if err := ls.store.SetMountParent(mount.name, mount.parent.chainID); err != nil {
			return err
		}
	}

	ls.mounts[mount.name] = mount

	return nil
}

func (ls *layerStore) initMount(graphID, parent, mountLabel string, initFunc MountInit) (string, error) {
	// Use "<graph-id>-init" to maintain compatibility with graph drivers
	// which are expecting this layer with this special name. If all
	// graph drivers can be updated to not rely on knowin about this layer
	// then the initID should be randomly generated.
	initID := fmt.Sprintf("%s-init", graphID)

	if err := ls.driver.Create(initID, parent, mountLabel); err != nil {

	}
	p, err := ls.driver.Get(initID, "")
	if err != nil {
		return "", err
	}

	if err := initFunc(p); err != nil {
		ls.driver.Put(initID)
		return "", err
	}

	if err := ls.driver.Put(initID); err != nil {
		return "", err
	}

	return initID, nil
}

func (ls *layerStore) Mount(name string, parent ChainID, mountLabel string, initFunc MountInit) (l RWLayer, err error) {
	ls.mountL.Lock()
	defer ls.mountL.Unlock()
	m, ok := ls.mounts[name]
	if ok {
		// Check if has path
		if err := ls.mount(m, mountLabel); err != nil {
			return nil, err
		}
		return m, nil
	}

	var pid string
	var p *roLayer
	if string(parent) != "" {
		p = ls.get(parent)
		if p == nil {
			return nil, ErrLayerDoesNotExist
		}
		pid = p.cacheID

		// Release parent chain if error
		defer func() {
			if err != nil {
				ls.layerL.Lock()
				ls.releaseLayer(p)
				ls.layerL.Unlock()
			}
		}()
	}

	mountID := name
	if runtime.GOOS != "windows" {
		// windows has issues if container ID doesn't match mount ID
		mountID = stringid.GenerateRandomID()
	}

	m = &mountedLayer{
		name:       name,
		parent:     p,
		mountID:    mountID,
		layerStore: ls,
	}

	if initFunc != nil {
		pid, err = ls.initMount(m.mountID, pid, mountLabel, initFunc)
		if err != nil {
			return nil, err
		}
		m.initID = pid
	}

	if err = ls.driver.Create(m.mountID, pid, ""); err != nil {
		return nil, err
	}

	if err = ls.saveMount(m); err != nil {
		return nil, err
	}

	if err = ls.mount(m, mountLabel); err != nil {
		return nil, err
	}

	return m, nil
}

func (ls *layerStore) Unmount(name string) error {
	ls.mountL.Lock()
	defer ls.mountL.Unlock()

	m := ls.mounts[name]
	if m == nil {
		return ErrMountDoesNotExist
	}

	m.activityCount--

	if err := ls.driver.Put(m.mountID); err != nil {
		return err
	}

	return nil
}

func (ls *layerStore) DeleteMount(name string) ([]Metadata, error) {
	ls.mountL.Lock()
	defer ls.mountL.Unlock()

	m := ls.mounts[name]
	if m == nil {
		return nil, ErrMountDoesNotExist
	}
	if m.activityCount > 0 {
		return nil, ErrActiveMount
	}

	delete(ls.mounts, name)

	if err := ls.driver.Remove(m.mountID); err != nil {
		logrus.Errorf("Error removing mounted layer %s: %s", m.name, err)
		return nil, err
	}

	if m.initID != "" {
		if err := ls.driver.Remove(m.initID); err != nil {
			logrus.Errorf("Error removing init layer %s: %s", m.name, err)
			return nil, err
		}
	}

	if err := ls.store.RemoveMount(m.name); err != nil {
		logrus.Errorf("Error removing mount metadata: %s: %s", m.name, err)
		return nil, err
	}

	ls.layerL.Lock()
	defer ls.layerL.Unlock()
	if m.parent != nil {
		return ls.releaseLayer(m.parent)
	}

	return []Metadata{}, nil
}

func (ls *layerStore) Changes(name string) ([]archive.Change, error) {
	ls.mountL.Lock()
	m := ls.mounts[name]
	ls.mountL.Unlock()
	if m == nil {
		return nil, ErrMountDoesNotExist
	}
	pid := m.initID
	if pid == "" && m.parent != nil {
		pid = m.parent.cacheID
	}
	return ls.driver.Changes(m.mountID, pid)
}

func (ls *layerStore) assembleTar(graphID string, metadata io.ReadCloser, size *int64) (io.ReadCloser, error) {
	type diffPathDriver interface {
		DiffPath(string) (string, func() error, error)
	}

	diffDriver, ok := ls.driver.(diffPathDriver)
	if !ok {
		diffDriver = &naiveDiffPathDriver{ls.driver}
	}

	// get our relative path to the container
	fsPath, releasePath, err := diffDriver.DiffPath(graphID)
	if err != nil {
		metadata.Close()
		return nil, err
	}

	pR, pW := io.Pipe()
	// this will need to be in a goroutine, as we are returning the stream of a
	// tar archive, but can not close the metadata reader early (when this
	// function returns)...
	go func() {
		defer releasePath()
		defer metadata.Close()

		metaUnpacker := storage.NewJSONUnpacker(metadata)
		upackerCounter := &unpackSizeCounter{metaUnpacker, size}
		fileGetter := storage.NewPathFileGetter(fsPath)
		logrus.Debugf("Assembling tar data for %s from %s", graphID, fsPath)
		ots := asm.NewOutputTarStream(fileGetter, upackerCounter)
		defer ots.Close()
		if _, err := io.Copy(pW, ots); err != nil {
			pW.CloseWithError(err)
			return
		}
		pW.Close()
	}()
	return pR, nil
}

type naiveDiffPathDriver struct {
	graphdriver.Driver
}

func (n *naiveDiffPathDriver) DiffPath(id string) (string, func() error, error) {
	p, err := n.Driver.Get(id, "")
	if err != nil {
		return "", nil, err
	}
	return p, func() error {
		return n.Driver.Put(id)
	}, nil
}
