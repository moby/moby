package graphdriver

import (
	"fmt"
	"time"

	"github.com/docker/docker/archive"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/log"
	"github.com/docker/docker/utils"
)

// GenericDriverWrapper takes a generic Driver and adds the
// capability of the following methods which it doesn't
// support on its own:
//     Diff(id, parent string) (archive.Archive, error)
//     Changes(id, parent string) ([]archive.Change, error)
//     ApplyDiff(id, parent string, diff archive.ArchiveReader) (bytes int64, err error)
//     DiffSize(id, parent string) (bytes int64, err error)
// Notably, the AUFS driver doesn't need to be wrapped like this.
type GenericDriverWrapper struct {
	GenericDriver
}

// NewGenericDriverWrapper returns a fully functional driver that wraps the given GenericDriver.
func NewGenericDriverWrapper(driver GenericDriver) Driver {
	return &GenericDriverWrapper{GenericDriver: driver}
}

// Diff produces an archive of the changes between the specified
// layer and its parent layer which may be "".
func (gdw *GenericDriverWrapper) Diff(id, parent string) (arch archive.Archive, err error) {
	driver := gdw.GenericDriver

	layerFs, err := driver.Get(id, "")
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			driver.Put(id)
		}
	}()

	if parent == "" {
		archive, err := archive.Tar(layerFs, archive.Uncompressed)
		if err != nil {
			return nil, err
		}
		return ioutils.NewReadCloserWrapper(archive, func() error {
			err := archive.Close()
			driver.Put(id)
			return err
		}), nil
	}

	parentFs, err := driver.Get(parent, "")
	if err != nil {
		return nil, err
	}
	defer driver.Put(parent)

	changes, err := archive.ChangesDirs(layerFs, parentFs)
	if err != nil {
		return nil, err
	}

	archive, err := archive.ExportChanges(layerFs, changes)
	if err != nil {
		return nil, err
	}

	return ioutils.NewReadCloserWrapper(archive, func() error {
		err := archive.Close()
		driver.Put(id)
		return err
	}), nil
}

// Changes produces a list of changes between the specified layer
// and its parent layer. If parent is "", then all changes will be ADD changes.
func (gdw *GenericDriverWrapper) Changes(id, parent string) ([]archive.Change, error) {
	driver := gdw.GenericDriver

	layerFs, err := driver.Get(id, "")
	if err != nil {
		return nil, err
	}
	defer driver.Put(id)

	parentFs := ""

	if parent != "" {
		parentFs, err = driver.Get(parent, "")
		if err != nil {
			return nil, err
		}
		defer driver.Put(parent)
	}

	return archive.ChangesDirs(layerFs, parentFs)
}

// ApplyDiff extracts the changeset from the given diff into the
// layer with the specified id and parent, returning the size of the
// new layer in bytes.
func (gdw *GenericDriverWrapper) ApplyDiff(id, parent string, diff archive.ArchiveReader) (bytes int64, err error) {
	driver := gdw.GenericDriver

	// Mount the root filesystem so we can apply the diff/layer.
	layerFs, err := driver.Get(id, "")
	if err != nil {
		return
	}
	defer driver.Put(id)

	start := time.Now().UTC()
	log.Debugf("Start untar layer")
	if err = archive.ApplyLayer(layerFs, diff); err != nil {
		return
	}
	log.Debugf("Untar time: %vs", time.Now().UTC().Sub(start).Seconds())

	if parent == "" {
		return utils.TreeSize(layerFs)
	}

	parentFs, err := driver.Get(parent, "")
	if err != nil {
		err = fmt.Errorf("Driver %s failed to get image parent %s: %s", driver, parent, err)
		return
	}
	defer driver.Put(parent)

	changes, err := archive.ChangesDirs(layerFs, parentFs)
	if err != nil {
		return
	}

	return archive.ChangesSize(layerFs, changes), nil
}

// DiffSize calculates the changes between the specified layer
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (gdw *GenericDriverWrapper) DiffSize(id, parent string) (bytes int64, err error) {
	driver := gdw.GenericDriver

	changes, err := gdw.Changes(id, parent)
	if err != nil {
		return
	}

	layerFs, err := driver.Get(id, "")
	if err != nil {
		return
	}
	defer driver.Put(id)

	return archive.ChangesSize(layerFs, changes), nil
}
