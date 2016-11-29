package xfer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/progress"
	"golang.org/x/net/context"
)

const maxDownloadAttempts = 5

// LayerDownloadManager figures out which layers need to be downloaded, then
// registers and downloads those, taking into account dependencies between
// layers.
type LayerDownloadManager struct {
	layerStore               layer.Store
	tm                       TransferManager
	cachedLayerRefsLock      sync.Mutex
	cachedLayerRefs          map[string]layer.Layer
	imagePullCheckpointsPath string
}

// SetConcurrency set the max concurrent downloads for each pull
func (ldm *LayerDownloadManager) SetConcurrency(concurrency int) {
	ldm.tm.SetConcurrency(concurrency)
}

// NewLayerDownloadManager returns a new LayerDownloadManager.
func NewLayerDownloadManager(layerStore layer.Store, concurrencyLimit int) *LayerDownloadManager {
	ldm := &LayerDownloadManager{
		layerStore:               layerStore,
		tm:                       NewTransferManager(concurrencyLimit, filepath.Join(os.Getenv("TMPDIR"), "resumable_layer_downloads")),
		cachedLayerRefs:          make(map[string]layer.Layer),
		imagePullCheckpointsPath: filepath.Join(os.Getenv("TMPDIR"), "resumable_image_downloads"),
	}

	ldm.loadLayerRefs()

	return ldm
}

type downloadTransfer struct {
	Transfer

	layerStore layer.Store
	layer      layer.Layer
	err        error
}

// result returns the layer resulting from the download, if the download
// and registration were successful.
func (d *downloadTransfer) result() (layer.Layer, error) {
	return d.layer, d.err
}

// A DownloadDescriptor references a layer that may need to be downloaded.
type DownloadDescriptor interface {
	// Key returns the key used to deduplicate downloads.
	Key() string
	// ID returns the ID for display purposes.
	ID() string
	// DiffID should return the DiffID for this layer, or an error
	// if it is unknown (for example, if it has not been downloaded
	// before).
	DiffID() (layer.DiffID, error)
	// Download is called to perform the download.
	Download(ctx context.Context, progressOutput progress.Output, cacheDir string) (io.ReadCloser, int64, error)
	// Close is called when the download manager is finished with this
	// descriptor and will not call Download again or read from the reader
	// that Download returned.
	Close()
}

// DownloadDescriptorWithRegistered is a DownloadDescriptor that has an
// additional Registered method which gets called after a downloaded layer is
// registered. This allows the user of the download manager to know the DiffID
// of each registered layer. This method is called if a cast to
// DownloadDescriptorWithRegistered is successful.
type DownloadDescriptorWithRegistered interface {
	DownloadDescriptor
	Registered(diffID layer.DiffID)
}

// CollectGarbage removes any cached layers associated with downloads that are
// still in progress
func (ldm *LayerDownloadManager) CollectGarbage() error {
	// we just flush the whole resumption cache and then save it
	ldm.cachedLayerRefsLock.Lock()
	for _, l := range ldm.cachedLayerRefs {
		layer.ReleaseAndLog(ldm.layerStore, l)
	}
	ldm.cachedLayerRefs = make(map[string]layer.Layer)
	ldm.cachedLayerRefsLock.Unlock()
	ldm.saveLayerRefs()

	return ldm.tm.CollectGarbage()
}

func (ldm *LayerDownloadManager) loadLayerRefs() {
	data, err := ioutil.ReadFile(ldm.imagePullCheckpointsPath)
	if err != nil {
		logrus.Warnf("failed to read image pull checkpoint: %s", err)
	}
	checkpoint := map[string]string{}
	err = json.Unmarshal(data, &checkpoint)
	if err != nil {
		logrus.Warnf("failed to unmarshal pull checkpoint: %s", err)
	}
	ldm.cachedLayerRefsLock.Lock()
	defer ldm.cachedLayerRefsLock.Unlock()
	for key, chainID := range checkpoint {
		layer, err := ldm.layerStore.Get(layer.ChainID(chainID))
		if err != nil {
			logrus.Warnf("failed to grab reference to %s during image pull checkpoint restore: %s", chainID, err)
		} else {
			ldm.cachedLayerRefs[key] = layer
		}
	}
}

// TODO: make atomic
func (ldm *LayerDownloadManager) saveLayerRefs() {
	checkpoint := map[string]string{}
	ldm.cachedLayerRefsLock.Lock()
	for key, layer := range ldm.cachedLayerRefs {
		checkpoint[key] = string(layer.ChainID())
	}
	ldm.cachedLayerRefsLock.Unlock()
	data, _ := json.Marshal(checkpoint)
	err := ioutil.WriteFile(ldm.imagePullCheckpointsPath, data, 0600)
	if err != nil {
		logrus.Warnf("failed to write out image pull checkpoint: %s", err)
	}
}

func (ldm *LayerDownloadManager) checkpointImage(resumptionCacheKey string, srcLayerRef layer.Layer) {
	// We grab and cache a new reference so that we don't lose it
	// when the download releases it. We need to do that before
	// Transfer.Release is called.
	layerRef, err := ldm.layerStore.Get(srcLayerRef.ChainID())
	// if for any reason we fail to get this reference, we just don't cache
	// this download and move on
	if err != nil {
		logrus.Warnf("failed to get reference to %s: %s", layerRef.ChainID(), err)
	} else {
		// Cache this reference so that we can resume from any of the layers in
		// the chain when someone tries to download an image with shared layers
		ldm.cachedLayerRefsLock.Lock()
		// If there was already an earlier checkpoint for this image, release and
		// replace it
		if lOld, ok := ldm.cachedLayerRefs[resumptionCacheKey]; ok {
			layer.ReleaseAndLog(ldm.layerStore, lOld)
		}
		ldm.cachedLayerRefs[resumptionCacheKey] = layerRef
		ldm.cachedLayerRefsLock.Unlock()
		ldm.saveLayerRefs()
	}
}

func (ldm *LayerDownloadManager) removeCheckpoint(resumptionCacheKey string) {
	ldm.cachedLayerRefsLock.Lock()
	if l, ok := ldm.cachedLayerRefs[resumptionCacheKey]; ok {
		layer.ReleaseAndLog(ldm.layerStore, l)
		delete(ldm.cachedLayerRefs, resumptionCacheKey)
	}
	ldm.cachedLayerRefsLock.Unlock()
	ldm.saveLayerRefs()
}

// descriptorsToCacheKey shortens the key so we don't end up with a crazy looking file
// this function needs to be cryptographically secure to prevent cache poisoning
func descriptorsToCacheKey(descriptors []DownloadDescriptor) string {
	hash := sha256.New()
	for _, descriptor := range descriptors {
		hash.Write([]byte(descriptor.Key()))
	}
	out := make([]byte, hash.Size()*2)
	hex.Encode(out, hash.Sum(nil))
	return string(out)
}

// Download is a blocking function which ensures the requested layers are
// present in the layer store. It uses the string returned by the Key method to
// deduplicate downloads. If a given layer is not already known to present in
// the layer store, and the key is not used by an in-progress download, the
// Download method is called to get the layer tar data. Layers are then
// registered in the appropriate order.  The caller must call the returned
// release function once it is is done with the returned RootFS object.
func (ldm *LayerDownloadManager) Download(ctx context.Context, initialRootFS image.RootFS, layers []DownloadDescriptor, progressOutput progress.Output) (image.RootFS, func(), error) {
	var (
		// the topLayer is the last layer that we were able to find in the
		// layerStore and didn't have to download
		topLayer       layer.Layer
		topDownload    *downloadTransfer
		watcher        *Watcher
		transferKey    = ""
		downloadsByKey = make(map[string]*downloadTransfer)
		// This channel is non-blocking and is used to receive layer references
		// from downloads if and when they fully download layers
		layerRefsCh = make(chan layer.Layer, len(layers))
	)

	// Look for a rootFS/layer we can resume from before trying to download layers.
	// This loop goes up to resumeLevel layers
	rootFS := initialRootFS
	resumeLevel := 0
	transferKey = ""
	for _, descriptor := range layers {
		diffID, err := descriptor.DiffID()
		if err == nil {
			// getRootFS is a temporary rootfs we create to check whether or
			// not we'll be able to find an instance of it in the layer store
			getRootFS := rootFS
			getRootFS.Append(diffID)
			l, err := ldm.layerStore.Get(getRootFS.ChainID())
			if err == nil {
				// Layer already exists.
				logrus.Debugf("Layer already exists: %s", descriptor.ID())
				progress.Update(progressOutput, descriptor.ID(), "Already exists")
				if topLayer != nil {
					layer.ReleaseAndLog(ldm.layerStore, topLayer)
				}
				topLayer = l
				rootFS.Append(diffID)
				resumeLevel++
				key := descriptor.Key()
				transferKey += key
			} else {
				// if we failed to get the layer for this level for any
				// reason, we give up and start downloading it instead
				break
			}
		}
	}

	// This loop starts at resumeLevel and continues for the rest of the layers
	for _, descriptor := range layers[resumeLevel:] {
		key := descriptor.Key()
		transferKey += key

		// Does this layer have the same data as a previous layer in
		// the stack? If so, avoid downloading it more than once.
		var topDownloadUncasted Transfer
		if existingDownload, ok := downloadsByKey[key]; ok {
			xferFunc := ldm.makeDownloadFuncFromDownload(descriptor, existingDownload, topDownload)
			defer topDownload.Transfer.Release(watcher)
			topDownloadUncasted, watcher = ldm.tm.Transfer(transferKey, xferFunc, progressOutput)
			topDownload = topDownloadUncasted.(*downloadTransfer)
			continue
		}

		// Layer is not known to exist - download and register it.
		progress.Update(progressOutput, descriptor.ID(), "Pulling fs layer")

		var xferFunc DoFunc
		if topDownload != nil {
			xferFunc = ldm.makeDownloadFunc(descriptor, "", topDownload, layerRefsCh)
			defer topDownload.Transfer.Release(watcher)
		} else {
			xferFunc = ldm.makeDownloadFunc(descriptor, rootFS.ChainID(), nil, layerRefsCh)
		}
		topDownloadUncasted, watcher = ldm.tm.Transfer(transferKey, xferFunc, progressOutput)
		topDownload = topDownloadUncasted.(*downloadTransfer)
		downloadsByKey[key] = topDownload
	}

	if topDownload == nil {
		// Nothing to download. This means we can just return
		return rootFS, func() {
			if topLayer != nil {
				layer.ReleaseAndLog(ldm.layerStore, topLayer)
			}
		}, nil
	}

	// A download's checkpoint is uniquely identified by the stack of descriptors it's for
	resumptionCacheKey := descriptorsToCacheKey(layers)
	logrus.Debugf("resumption cache key for this download: %s", resumptionCacheKey)

	// Won't be using the list built up so far - will generate it
	// from downloaded layers instead because we might not know the DiffIDs
	// ahead of time.
	rootFS.DiffIDs = []layer.DiffID{}

	defer func() {
		if topLayer != nil {
			layer.ReleaseAndLog(ldm.layerStore, topLayer)
		}
	}()

	// cache layers as they come in
done:
	for {
		select {
		case layerRef := <-layerRefsCh:
			// any layers received here will be new, so we want to
			// checkpoint this download to each layer as they come in
			ldm.checkpointImage(resumptionCacheKey, layerRef)
		case <-ctx.Done():
			// we don't do anything special upon cancellation because we've been
			// checkpointing the progress as layers were coming in
			topDownload.Transfer.Release(watcher)
			return rootFS, func() {}, ctx.Err()
		case <-topDownload.Done():
			break done
		}
	}

	l, err := topDownload.result()
	if err != nil {
		topDownload.Transfer.Release(watcher)
		return rootFS, func() {}, err
	}

	// Construct the final rootFS form the diffIDs of the layers by walking up the
	// chain of parents. We must do this exactly len(layers) times, so we don't
	// include the base layer on Windows.
	for range layers {
		if l == nil {
			topDownload.Transfer.Release(watcher)
			return rootFS, func() {}, errors.New("internal error: too few parent layers")
		}
		rootFS.DiffIDs = append([]layer.DiffID{l.DiffID()}, rootFS.DiffIDs...)
		l = l.Parent()
	}

	// Now we've successfully downloaded an image so we remove any layer reference
	// that we've previously cached for resumption of this image. If there were
	// two pulls for the same image, we don't care - we just clean this up because
	// we know at least one download has succeeded and we don't need this cache any more.
	ldm.removeCheckpoint(resumptionCacheKey)

	return rootFS, func() { topDownload.Transfer.Release(watcher) }, err
}

// makeDownloadFunc returns a function that performs the layer download and
// registration. If parentDownload is non-nil, it waits for that download to
// complete before the registration step, and registers the downloaded data
// on top of parentDownload's resulting layer. Otherwise, it registers the
// layer on top of the ChainID given by parentLayer.
func (ldm *LayerDownloadManager) makeDownloadFunc(descriptor DownloadDescriptor, parentLayer layer.ChainID, parentDownload *downloadTransfer, layerRefsCh chan layer.Layer) DoFunc {
	return func(progressChan chan<- progress.Progress, start <-chan struct{}, inactive chan<- struct{}, cachePath string) Transfer {
		d := &downloadTransfer{
			Transfer:   NewTransfer(),
			layerStore: ldm.layerStore,
		}

		go func() {
			defer func() {
				close(progressChan)
			}()

			progressOutput := progress.ChanOutput(progressChan)

			select {
			case <-start:
			default:
				progress.Update(progressOutput, descriptor.ID(), "Waiting")
				<-start
			}

			if parentDownload != nil {
				// Did the parent download already fail or get
				// cancelled?
				select {
				case <-parentDownload.Done():
					_, err := parentDownload.result()
					if err != nil {
						d.err = err
						return
					}
				default:
				}
			}

			var (
				downloadReader io.ReadCloser
				size           int64
				err            error
				retries        int
			)

			defer descriptor.Close()

			for {
				downloadReader, size, err = descriptor.Download(d.Transfer.Context(), progressOutput, cachePath)
				if err == nil {
					break
				}

				// If an error was returned because the context
				// was cancelled, we shouldn't retry.
				select {
				case <-d.Transfer.Context().Done():
					d.err = err
					return
				default:
				}

				retries++
				if _, isDNR := err.(DoNotRetry); isDNR || retries == maxDownloadAttempts {
					logrus.Errorf("Download failed: %v", err)
					d.err = err
					return
				}

				logrus.Errorf("Download failed, retrying: %v", err)
				delay := retries * 5
				ticker := time.NewTicker(time.Second)

			selectLoop:
				for {
					progress.Updatef(progressOutput, descriptor.ID(), "Retrying in %d second%s", delay, (map[bool]string{true: "s"})[delay != 1])
					select {
					case <-ticker.C:
						delay--
						if delay == 0 {
							ticker.Stop()
							break selectLoop
						}
					case <-d.Transfer.Context().Done():
						ticker.Stop()
						d.err = errors.New("download cancelled during retry delay")
						return
					}

				}
			}

			close(inactive)

			if parentDownload != nil {
				select {
				case <-d.Transfer.Context().Done():
					d.err = errors.New("layer registration cancelled")
					downloadReader.Close()
					return
				case <-parentDownload.Done():
				}

				l, err := parentDownload.result()
				if err != nil {
					d.err = err
					downloadReader.Close()
					return
				}
				parentLayer = l.ChainID()
			}

			reader := progress.NewProgressReader(ioutils.NewCancelReadCloser(d.Transfer.Context(), downloadReader), progressOutput, 0, size, descriptor.ID(), "Extracting")
			defer reader.Close()

			inflatedLayerData, err := archive.DecompressStream(reader)
			if err != nil {
				d.err = fmt.Errorf("could not get decompression stream: %v", err)
				return
			}

			var src distribution.Descriptor
			if fs, ok := descriptor.(distribution.Describable); ok {
				src = fs.Descriptor()
			}
			if ds, ok := d.layerStore.(layer.DescribableStore); ok {
				d.layer, err = ds.RegisterWithDescriptor(inflatedLayerData, parentLayer, src)
			} else {
				d.layer, err = d.layerStore.Register(inflatedLayerData, parentLayer)
			}
			if err != nil {
				select {
				case <-d.Transfer.Context().Done():
					d.err = errors.New("layer registration cancelled")
				default:
					d.err = fmt.Errorf("failed to register layer: %v", err)
				}
				return
			} else {
				// send all successful layers on the channel
				layerRefsCh <- d.layer
			}

			progress.Update(progressOutput, descriptor.ID(), "Pull complete")
			withRegistered, hasRegistered := descriptor.(DownloadDescriptorWithRegistered)
			if hasRegistered {
				withRegistered.Registered(d.layer.DiffID())
			}

			// If we've gotten this far, mark the transfer as successful so the
			// resumable download cache directory can be cleaned up
			if d.layer != nil {
				d.Transfer.SetSuccess()
			}

			// Doesn't actually need to be its own goroutine, but
			// done like this so we can defer close(c).
			go func() {
				<-d.Transfer.Released()
				// we release the layer here because the next download has already
				// taken a reference to it, so we don't need this one
				if d.layer != nil {
					layer.ReleaseAndLog(d.layerStore, d.layer)
				}
			}()
		}()

		return d
	}
}

// makeDownloadFuncFromDownload returns a function that performs the layer
// registration when the layer data is coming from an existing download. It
// waits for sourceDownload and parentDownload to complete, and then
// reregisters the data from sourceDownload's top layer on top of
// parentDownload. This function does not log progress output because it would
// interfere with the progress reporting for sourceDownload, which has the same
// Key.
func (ldm *LayerDownloadManager) makeDownloadFuncFromDownload(descriptor DownloadDescriptor, sourceDownload *downloadTransfer, parentDownload *downloadTransfer) DoFunc {
	return func(progressChan chan<- progress.Progress, start <-chan struct{}, inactive chan<- struct{}, cachePath string) Transfer {
		d := &downloadTransfer{
			Transfer:   NewTransfer(),
			layerStore: ldm.layerStore,
		}

		go func() {
			defer func() {
				close(progressChan)
			}()

			<-start

			close(inactive)

			select {
			case <-d.Transfer.Context().Done():
				d.err = errors.New("layer registration cancelled")
				return
			case <-parentDownload.Done():
			}

			l, err := parentDownload.result()
			if err != nil {
				d.err = err
				return
			}
			parentLayer := l.ChainID()

			// sourceDownload should have already finished if
			// parentDownload finished, but wait for it explicitly
			// to be sure.
			select {
			case <-d.Transfer.Context().Done():
				d.err = errors.New("layer registration cancelled")
				return
			case <-sourceDownload.Done():
			}

			l, err = sourceDownload.result()
			if err != nil {
				d.err = err
				return
			}

			layerReader, err := l.TarStream()
			if err != nil {
				d.err = err
				return
			}
			defer layerReader.Close()

			var src distribution.Descriptor
			if fs, ok := l.(distribution.Describable); ok {
				src = fs.Descriptor()
			}
			if ds, ok := d.layerStore.(layer.DescribableStore); ok {
				d.layer, err = ds.RegisterWithDescriptor(layerReader, parentLayer, src)
			} else {
				d.layer, err = d.layerStore.Register(layerReader, parentLayer)
			}
			if err != nil {
				d.err = fmt.Errorf("failed to register layer: %v", err)
				return
			}

			withRegistered, hasRegistered := descriptor.(DownloadDescriptorWithRegistered)
			if hasRegistered {
				withRegistered.Registered(d.layer.DiffID())
			}

			// Doesn't actually need to be its own goroutine, but
			// done like this so we can defer close(c).
			go func() {
				<-d.Transfer.Released()
				if d.layer != nil {
					layer.ReleaseAndLog(d.layerStore, d.layer)
				}
			}()
		}()

		return d
	}
}
