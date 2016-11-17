package layer

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/plugingetter"
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

// StoreOptions are the options used to create a new Store instance
type StoreOptions struct {
	StorePath                 string
	MetadataStorePathTemplate string
	GraphDriver               string
	GraphDriverOptions        []string
	UIDMaps                   []idtools.IDMap
	GIDMaps                   []idtools.IDMap
	PluginGetter              plugingetter.PluginGetter
}

// NewStoreFromOptions creates a new Store instance
func NewStoreFromOptions(options StoreOptions) (Store, error) {
	driver, err := graphdriver.New(
		options.StorePath,
		options.GraphDriver,
		options.GraphDriverOptions,
		options.UIDMaps,
		options.GIDMaps,
		options.PluginGetter)
	if err != nil {
		return nil, fmt.Errorf("error initializing graphdriver: %v", err)
	}
	logrus.Debugf("Using graph driver %s", driver)

	fms, err := NewFSMetadataStore(fmt.Sprintf(options.MetadataStorePathTemplate, driver))
	if err != nil {
		return nil, err
	}

	return NewStoreFromGraphDriver(fms, driver)
}

// NewStoreFromGraphDriver creates a new Store instance using the provided
// metadata store and graph driver. The metadata store will be used to restore
// the Store.
func NewStoreFromGraphDriver(store MetadataStore, driver graphdriver.Driver) (Store, error) {
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
			continue
		}
		if l.parent != nil {
			l.parent.referenceCount++
		}
	}

	for _, mount := range mounts {
		if _, err := ls.loadMount(mount); err != nil {
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
		return nil, fmt.Errorf("failed to get diff id for %s: %s", layer, err)
	}

	size, err := ls.store.GetSize(layer)
	if err != nil {
		return nil, fmt.Errorf("failed to get size for %s: %s", layer, err)
	}

	cacheID, err := ls.store.GetCacheID(layer)
	if err != nil {
		return nil, fmt.Errorf("failed to get cache id for %s: %s", layer, err)
	}

	parent, err := ls.store.GetParent(layer)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent for %s: %s", layer, err)
	}

	descriptor, err := ls.store.GetDescriptor(layer)
	if err != nil {
		return nil, fmt.Errorf("failed to get descriptor for %s: %s", layer, err)
	}

	cl = &roLayer{
		chainID:    layer,
		diffID:     diff,
		size:       size,
		cacheID:    cacheID,
		layerStore: ls,
		references: map[Layer]struct{}{},
		descriptor: descriptor,
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

func (ls *layerStore) loadMount(mount string) (*mountedLayer, error) {
	if ml, ok := ls.mounts[mount]; ok {
		return ml, nil
	}

	mountID, err := ls.store.GetMountID(mount)
	if err != nil {
		return nil, err
	}

	initID, err := ls.store.GetInitID(mount)
	if err != nil {
		return nil, err
	}

	parent, err := ls.store.GetMountParent(mount)
	if err != nil {
		return nil, err
	}

	sharedRootFs, err := ls.store.GetMountShared(mount)
	if err != nil {
		return nil, err
	}

	initName, err := ls.store.GetInitName(mount)
	if err != nil {
		return nil, err
	}

	ml := &mountedLayer{
		name:         mount,
		mountID:      mountID,
		initID:       initID,
		sharedRootFs: sharedRootFs,
		layerStore:   ls,
		references:   map[RWLayer]*referencedRWLayer{},
	}

	if parent != "" {
		p, err := ls.loadLayer(parent)
		if err != nil {
			return nil, err
		}
		ml.parent = p

		p.referenceCount++
	}

	if initName != "" {
		initml, err := ls.loadMount(initName)
		if err != nil {
			ml.parent.referenceCount--
			return nil, err
		}

		ml.initRWLayer = initml.getReference()
	}

	ls.mounts[ml.name] = ml

	return ml, nil
}

func (ls *layerStore) applyTar(tx MetadataTransaction, ts io.Reader, parent string, layer *roLayer) error {
	digester := digest.Canonical.New()
	tr := io.TeeReader(ts, digester.Hash())

	tsw, err := tx.TarSplitWriter(true)
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

	applySize, err := ls.driver.ApplyDiff(layer.cacheID, parent, rdr)
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
	return ls.registerWithDescriptor(ts, parent, distribution.Descriptor{})
}

func (ls *layerStore) registerWithDescriptor(ts io.Reader, parent ChainID, descriptor distribution.Descriptor) (Layer, error) {
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
				ls.releaseLayerLock(p)
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
		descriptor:     descriptor,
	}

	if err = ls.driver.Create(layer.cacheID, pid, nil); err != nil {
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
	ls.layerL.Lock()
	defer ls.layerL.Unlock()

	layer := ls.getWithoutLock(l)
	if layer == nil {
		return nil, ErrLayerDoesNotExist
	}

	return layer.getReference(), nil
}

func (ls *layerStore) Map() map[ChainID]Layer {
	ls.layerL.Lock()
	defer ls.layerL.Unlock()

	layers := map[ChainID]Layer{}

	for k, v := range ls.layerMap {
		layers[k] = v
	}

	return layers
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

func (ls *layerStore) releaseLayerLock(l *roLayer) ([]Metadata, error) {
	ls.layerL.Lock()
	defer ls.layerL.Unlock()
	return ls.releaseLayer(l)
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

// This should be called with mounts lock held.
func (ls *layerStore) createRWLayerLocked(name string, parent *roLayer, mountID string, mountLabel string, storageOpt map[string]string, initLayerID string, initRWLayer RWLayer, sharedRootFs bool) (RWLayer, error) {
	m, ok := ls.mounts[name]
	if ok {
		return nil, ErrMountNameConflict
	}

	m = &mountedLayer{
		name:         name,
		parent:       parent,
		mountID:      mountID,
		layerStore:   ls,
		references:   map[RWLayer]*referencedRWLayer{},
		initID:       initLayerID,
		initRWLayer:  initRWLayer,
		sharedRootFs: sharedRootFs,
	}

	pid := ""
	if m.initID != "" {
		pid = m.initID
	} else if parent != nil {
		pid = parent.cacheID
	}

	createOpts := &graphdriver.CreateOpts{
		MountLabel: mountLabel,
		StorageOpt: storageOpt,
		Shared:     sharedRootFs,
	}
	err := ls.driver.CreateReadWrite(m.mountID, pid, createOpts)
	if err != nil {
		return nil, err
	}

	if err := ls.saveMount(m); err != nil {
		return nil, err
	}

	return m.getReference(), nil
}

func (ls *layerStore) initMount(initRWLayer RWLayer, initFunc MountInit, mountLabel string) error {
	mountPath, err := initRWLayer.Mount(mountLabel)
	if err != nil {
		return err
	}

	if err := initFunc(mountPath); err != nil {
		initRWLayer.Unmount()
		return err
	}

	if err := initRWLayer.Unmount(); err != nil {
		return err
	}
	return nil
}

// Should be called with mounts lock held.
func (ls *layerStore) createRWInitLayerLocked(name string, parent ChainID, mountID string, initFunc MountInit, mountLabel string, storageOpt map[string]string) (RWLayer, error) {
	var initp *roLayer
	var err error
	var initRWLayer RWLayer

	if string(parent) != "" {
		initp = ls.get(parent)
		if initp == nil {
			return nil, ErrLayerDoesNotExist
		}
		// Release parent chain if error
		defer func() {
			if err != nil {
				ls.releaseLayerLock(initp)
			}
		}()
	}

	initRWLayer, err = ls.createRWLayerLocked(name, initp, mountID, mountLabel, storageOpt, "", nil, false)
	if err != nil {
		return nil, err
	}

	err = ls.initMount(initRWLayer, initFunc, mountLabel)
	if err != nil && err != ErrMountNameConflict {
		ls.releaseRWLayerLocked(initRWLayer)
		return nil, err
	}

	return initRWLayer, nil
}

func (ls *layerStore) CreateRWLayer(name string, parent ChainID, opts *CreateRWLayerOpts) (RWLayer, error) {
	var (
		storageOpt   map[string]string
		initFunc     MountInit
		mountLabel   string
		sharedRootFs bool
	)

	if opts != nil {
		mountLabel = opts.MountLabel
		storageOpt = opts.StorageOpt
		initFunc = opts.InitFunc
		sharedRootFs = opts.SharedRootFs
	}

	ls.mountL.Lock()
	defer ls.mountL.Unlock()
	_, ok := ls.mounts[name]
	if ok {
		return nil, ErrMountNameConflict
	}

	var err error
	var initMountID, initMountName string
	var initRWLayer, rwLayer RWLayer
	var p *roLayer
	if string(parent) != "" {
		// This is reference for actual layer and not -init layer.
		p = ls.get(parent)
		if p == nil {
			return nil, ErrLayerDoesNotExist
		}

		// Release parent chain when done.
		defer func() {
			if err != nil {
				ls.releaseLayerLock(p)
			}
		}()
	} else if sharedRootFs {
		return nil, fmt.Errorf("Need a valid parent layer when shared rootfs is true")
	}

	mountID := ls.mountID(name)

	// First create init RWLayer

	if initFunc != nil {
		// Use "<id>-init" to maintain compatibility with graph drivers
		// which are expecting this layer with this special name. If
		// all graph drivers can be updated to not rely on knowing
		// about this layer then the initID should be randomly
		// generated.
		if sharedRootFs {
			initMountName = fmt.Sprintf("%s-init", digest.Digest(string(parent)).Hex())
			initMountID = fmt.Sprintf("%s-init", p.cacheID)
			initRWLayer, err = ls.getRWLayerLocked(initMountName)
			if err != nil && err != ErrMountDoesNotExist {
				return nil, err
			}

		} else {
			initMountName = fmt.Sprintf("%s-init", name)
			initMountID = fmt.Sprintf("%s-init", mountID)
		}
		if initRWLayer == nil {
			initRWLayer, err = ls.createRWInitLayerLocked(initMountName, parent, initMountID, initFunc, mountLabel, storageOpt)
			if err != nil {
				return nil, err
			}

			// Release init layer in case of error
			defer func() {
				if err != nil {
					ls.releaseRWLayerLocked(initRWLayer)
				}
			}()
		}
	}

	rwLayer, err = ls.createRWLayerLocked(name, p, mountID, mountLabel, storageOpt, initMountID, initRWLayer, sharedRootFs)
	if err != nil {
		return nil, err
	}

	return rwLayer, nil
}

func (ls *layerStore) getRWLayerLocked(id string) (RWLayer, error) {
	mount, ok := ls.mounts[id]
	if !ok {
		return nil, ErrMountDoesNotExist
	}

	return mount.getReference(), nil
}

func (ls *layerStore) GetRWLayer(id string) (RWLayer, error) {
	ls.mountL.Lock()
	defer ls.mountL.Unlock()
	return ls.getRWLayerLocked(id)
}

func (ls *layerStore) GetMountID(id string) (string, error) {
	ls.mountL.Lock()
	defer ls.mountL.Unlock()
	mount, ok := ls.mounts[id]
	if !ok {
		return "", ErrMountDoesNotExist
	}
	logrus.Debugf("GetMountID id: %s -> mountID: %s", id, mount.mountID)

	return mount.mountID, nil
}

func (ls *layerStore) releaseRWLayerLocked(l RWLayer) ([]Metadata, error) {
	m, ok := ls.mounts[l.Name()]
	if !ok {
		return []Metadata{}, nil
	}

	if err := m.deleteReference(l); err != nil {
		return nil, err
	}

	if m.hasReferences() {
		return []Metadata{}, nil
	}

	if err := ls.driver.Remove(m.mountID); err != nil {
		logrus.Errorf("Error removing mounted layer %s: %s", m.name, err)
		m.retakeReference(l)
		return nil, err
	}

	if err := ls.store.RemoveMount(m.name); err != nil {
		logrus.Errorf("Error removing mount metadata: %s: %s", m.name, err)
		m.retakeReference(l)
		return nil, err
	}

	delete(ls.mounts, m.Name())

	if m.parent != nil {
		return ls.releaseLayerLock(m.parent)
	}

	return []Metadata{}, nil
}

func (ls *layerStore) releaseRWLayer(l RWLayer) ([]Metadata, error) {
	ls.mountL.Lock()
	defer ls.mountL.Unlock()
	return ls.releaseRWLayerLocked(l)
}

func (ls *layerStore) ReleaseRWLayer(l RWLayer) ([]Metadata, error) {
	ls.mountL.Lock()
	defer ls.mountL.Unlock()
	m, ok := ls.mounts[l.Name()]
	if !ok {
		return []Metadata{}, nil
	}

	initRWLayer := m.initRWLayer

	metadata, err := ls.releaseRWLayerLocked(l)
	if err != nil {
		return metadata, err
	}

	// Delete -init RWLayer.
	if initRWLayer != nil {
		if metadata, err = ls.releaseRWLayerLocked(initRWLayer); err != nil {
			return metadata, err
		}
	}

	return metadata, nil
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

	if mount.initRWLayer != nil {
		if err := ls.store.SetInitName(mount.name, mount.initRWLayer.Name()); err != nil {
			return err
		}
	}

	if mount.parent != nil {
		if err := ls.store.SetMountParent(mount.name, mount.parent.chainID); err != nil {
			return err
		}
	}

	if mount.sharedRootFs {
		if err := ls.store.SetMountShared(mount.name); err != nil {
			return err
		}
	}

	ls.mounts[mount.name] = mount

	return nil
}

func (ls *layerStore) assembleTarTo(graphID string, metadata io.ReadCloser, size *int64, w io.Writer) error {
	diffDriver, ok := ls.driver.(graphdriver.DiffGetterDriver)
	if !ok {
		diffDriver = &naiveDiffPathDriver{ls.driver}
	}

	defer metadata.Close()

	// get our relative path to the container
	fileGetCloser, err := diffDriver.DiffGetter(graphID)
	if err != nil {
		return err
	}
	defer fileGetCloser.Close()

	metaUnpacker := storage.NewJSONUnpacker(metadata)
	upackerCounter := &unpackSizeCounter{metaUnpacker, size}
	logrus.Debugf("Assembling tar data for %s", graphID)
	return asm.WriteOutputTarStream(fileGetCloser, upackerCounter, w)
}

func (ls *layerStore) Cleanup() error {
	return ls.driver.Cleanup()
}

func (ls *layerStore) DriverStatus() [][2]string {
	return ls.driver.Status()
}

func (ls *layerStore) DriverName() string {
	return ls.driver.String()
}

type naiveDiffPathDriver struct {
	graphdriver.Driver
}

type fileGetPutter struct {
	storage.FileGetter
	driver graphdriver.Driver
	id     string
}

func (w *fileGetPutter) Close() error {
	return w.driver.Put(w.id)
}

func (n *naiveDiffPathDriver) DiffGetter(id string) (graphdriver.FileGetCloser, error) {
	p, err := n.Driver.Get(id, "")
	if err != nil {
		return nil, err
	}
	return &fileGetPutter{storage.NewPathFileGetter(p), n.Driver, id}, nil
}
