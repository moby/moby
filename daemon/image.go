package daemon // import "github.com/docker/docker/daemon"

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/container"
	daemonevents "github.com/docker/docker/daemon/events"
	"github.com/docker/docker/distribution/metadata"
	"github.com/docker/docker/distribution/xfer"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	dockerreference "github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/libtrust"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// errImageDoesNotExist is error returned when no image can be found for a reference.
type errImageDoesNotExist struct {
	ref reference.Reference
}

func (e errImageDoesNotExist) Error() string {
	ref := e.ref
	if named, ok := ref.(reference.Named); ok {
		ref = reference.TagNameOnly(named)
	}
	return fmt.Sprintf("No such image: %s", reference.FamiliarString(ref))
}

func (e errImageDoesNotExist) NotFound() {}

// GetImageIDAndOS returns an image ID and operating system corresponding to the image referred to by
// refOrID.
// called from list.go foldFilter()
func (i imageService) GetImageIDAndOS(refOrID string) (image.ID, string, error) {
	ref, err := reference.ParseAnyReference(refOrID)
	if err != nil {
		return "", "", errdefs.InvalidParameter(err)
	}
	namedRef, ok := ref.(reference.Named)
	if !ok {
		digested, ok := ref.(reference.Digested)
		if !ok {
			return "", "", errImageDoesNotExist{ref}
		}
		id := image.IDFromDigest(digested.Digest())
		if img, err := i.imageStore.Get(id); err == nil {
			return id, img.OperatingSystem(), nil
		}
		return "", "", errImageDoesNotExist{ref}
	}

	if digest, err := i.referenceStore.Get(namedRef); err == nil {
		// Search the image stores to get the operating system, defaulting to host OS.
		id := image.IDFromDigest(digest)
		if img, err := i.imageStore.Get(id); err == nil {
			return id, img.OperatingSystem(), nil
		}
	}

	// Search based on ID
	if id, err := i.imageStore.Search(refOrID); err == nil {
		img, err := i.imageStore.Get(id)
		if err != nil {
			return "", "", errImageDoesNotExist{ref}
		}
		return id, img.OperatingSystem(), nil
	}

	return "", "", errImageDoesNotExist{ref}
}

// GetImage returns an image corresponding to the image referred to by refOrID.
func (i *imageService) GetImage(refOrID string) (*image.Image, error) {
	imgID, _, err := i.GetImageIDAndOS(refOrID)
	if err != nil {
		return nil, err
	}
	return i.imageStore.Get(imgID)
}

type containerStore interface {
	// used by image delete
	First(container.StoreFilter) *container.Container
	// used by image prune, and image list
	List() []*container.Container
	// TODO: remove, only used for CommitBuildStep
	Get(string) *container.Container
}

type imageService struct {
	eventsService   *daemonevents.Events
	containers      containerStore
	downloadManager *xfer.LayerDownloadManager
	uploadManager   *xfer.LayerUploadManager

	// TODO: should accept a trust service instead of a key
	trustKey libtrust.PrivateKey

	registryService           registry.Service
	referenceStore            dockerreference.Store
	distributionMetadataStore metadata.Store
	imageStore                image.Store
	layerStores               map[string]layer.Store // By operating system

	pruneRunning int32
}

// called from info.go
func (i *imageService) CountImages() int {
	return len(i.imageStore.Map())
}

// called from list.go to filter containers
func (i *imageService) Children(id image.ID) []image.ID {
	return i.imageStore.Children(id)
}

// TODO: accept an opt struct instead of container?
// called from create.go
func (i *imageService) GetRWLayer(container *container.Container, initFunc layer.MountInit) (layer.RWLayer, error) {
	var layerID layer.ChainID
	if container.ImageID != "" {
		img, err := i.imageStore.Get(container.ImageID)
		if err != nil {
			return nil, err
		}
		layerID = img.RootFS.ChainID()
	}

	rwLayerOpts := &layer.CreateRWLayerOpts{
		MountLabel: container.MountLabel,
		InitFunc:   initFunc,
		StorageOpt: container.HostConfig.StorageOpt,
	}

	// Indexing by OS is safe here as validation of OS has already been performed in create() (the only
	// caller), and guaranteed non-nil
	return i.layerStores[container.OS].CreateRWLayer(container.ID, layerID, rwLayerOpts)
}

// called from daemon.go Daemon.restore(), and Daemon.containerExport()
func (i *imageService) GetRWLayerByID(cid string, os string) (layer.RWLayer, error) {
	return i.layerStores[os].GetRWLayer(cid)
}

// called from info.go
func (i *imageService) GraphDriverStatuses() map[string][][2]string {
	result := make(map[string][][2]string)
	for os, store := range i.layerStores {
		result[os] = store.DriverStatus()
	}
	return result
}

// called from daemon.go Daemon.Shutdown(), and Daemon.Cleanup() (cleanup is actually continerCleanup)
func (i *imageService) GetContainerMountID(cid string, os string) (string, error) {
	return i.layerStores[os].GetMountID(cid)
}

// called from daemon.go Daemon.Shutdown()
func (i *imageService) Cleanup() {
	for os, ls := range i.layerStores {
		if ls != nil {
			if err := ls.Cleanup(); err != nil {
				logrus.Errorf("Error during layer Store.Cleanup(): %v %s", err, os)
			}
		}
	}
}

// moved from Daemon.GraphDriverName, multiple calls
func (i *imageService) GraphDriverForOS(os string) string {
	return i.layerStores[os].DriverName()
}

// called from delete.go Daemon.cleanupContainer(), and Daemon.containerExport()
func (i *imageService) ReleaseContainerLayer(rwlayer layer.RWLayer, containerOS string) error {
	metadata, err := i.layerStores[containerOS].ReleaseRWLayer(rwlayer)
	layer.LogReleaseMetadata(metadata)
	if err != nil && err != layer.ErrMountDoesNotExist && !os.IsNotExist(errors.Cause(err)) {
		return errors.Wrapf(err, "driver %q failed to remove root filesystem",
			i.layerStores[containerOS].DriverName())
	}
	return nil
}

// called from disk_usage.go
func (i *imageService) LayerDiskUsage(ctx context.Context) (int64, error) {
	var allLayersSize int64
	layerRefs := i.getLayerRefs()
	for _, ls := range i.layerStores {
		allLayers := ls.Map()
		for _, l := range allLayers {
			select {
			case <-ctx.Done():
				return allLayersSize, ctx.Err()
			default:
				size, err := l.DiffSize()
				if err == nil {
					if _, ok := layerRefs[l.ChainID()]; ok {
						allLayersSize += size
					} else {
						logrus.Warnf("found leaked image layer %v", l.ChainID())
					}
				} else {
					logrus.Warnf("failed to get diff size for layer %v", l.ChainID())
				}
			}
		}
	}
	return allLayersSize, nil
}

func (i *imageService) getLayerRefs() map[layer.ChainID]int {
	tmpImages := i.imageStore.Map()
	layerRefs := map[layer.ChainID]int{}
	for id, img := range tmpImages {
		dgst := digest.Digest(id)
		if len(i.referenceStore.References(dgst)) == 0 && len(i.imageStore.Children(id)) != 0 {
			continue
		}

		rootFS := *img.RootFS
		rootFS.DiffIDs = nil
		for _, id := range img.RootFS.DiffIDs {
			rootFS.Append(id)
			chid := rootFS.ChainID()
			layerRefs[chid]++
		}
	}

	return layerRefs
}

// LogImageEvent generates an event related to an image with only the default attributes.
func (i *imageService) LogImageEvent(imageID, refName, action string) {
	i.LogImageEventWithAttributes(imageID, refName, action, map[string]string{})
}

// LogImageEventWithAttributes generates an event related to an image with specific given attributes.
func (i *imageService) LogImageEventWithAttributes(imageID, refName, action string, attributes map[string]string) {
	img, err := i.GetImage(imageID)
	if err == nil && img.Config != nil {
		// image has not been removed yet.
		// it could be missing if the event is `delete`.
		copyAttributes(attributes, img.Config.Labels)
	}
	if refName != "" {
		attributes["name"] = refName
	}
	actor := events.Actor{
		ID:         imageID,
		Attributes: attributes,
	}

	i.eventsService.Log(action, events.ImageEventType, actor)
}
