package graphdriver // import "github.com/docker/docker/daemon/graphdriver"

import (
	"io"
	"time"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/sirupsen/logrus"
)

var (
	// ApplyUncompressedLayer defines the unpack method used by the graph
	// driver.
	ApplyUncompressedLayer = chrootarchive.ApplyUncompressedLayer
)

// NaiveDiffDriver takes a ProtoDriver and adds the
// capability of the Diffing methods on the local file system,
// which it may or may not support on its own. See the comment
// on the exported NewNaiveDiffDriver function below.
type NaiveDiffDriver struct {
	ProtoDriver
	IDMap idtools.IdentityMapping
	// If true, allow ApplyDiff to succeed in spite of failures to set
	// extended attributes on the unpacked files due to the destination
	// filesystem not supporting them or a lack of permissions. The
	// resulting unpacked layer may be subtly broken.
	BestEffortXattrs bool
}

// NewNaiveDiffDriver returns a fully functional driver that wraps the
// given ProtoDriver and adds the capability of the following methods which
// it may or may not support on its own:
//
//	Diff(id, parent string) (archive.Archive, error)
//	Changes(id, parent string) ([]archive.Change, error)
//	ApplyDiff(id, parent string, diff archive.Reader) (size int64, err error)
//	DiffSize(id, parent string) (size int64, err error)
func NewNaiveDiffDriver(driver ProtoDriver, idMap idtools.IdentityMapping) Driver {
	return &NaiveDiffDriver{ProtoDriver: driver,
		IDMap: idMap}
}

// Diff produces an archive of the changes between the specified
// layer and its parent layer which may be "".
func (gdw *NaiveDiffDriver) Diff(id, parent string) (arch io.ReadCloser, err error) {
	startTime := time.Now()
	driver := gdw.ProtoDriver

	layerRootFs, err := driver.Get(id, "")
	if err != nil {
		return nil, err
	}
	layerFs := layerRootFs

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

	archive, err := archive.ExportChanges(layerFs, changes, gdw.IDMap)
	if err != nil {
		return nil, err
	}

	return ioutils.NewReadCloserWrapper(archive, func() error {
		err := archive.Close()
		driver.Put(id)

		// NaiveDiffDriver compares file metadata with parent layers. Parent layers
		// are extracted from tar's with full second precision on modified time.
		// We need this hack here to make sure calls within same second receive
		// correct result.
		time.Sleep(time.Until(startTime.Truncate(time.Second).Add(time.Second)))
		return err
	}), nil
}

// Changes produces a list of changes between the specified layer
// and its parent layer. If parent is "", then all changes will be ADD changes.
func (gdw *NaiveDiffDriver) Changes(id, parent string) ([]archive.Change, error) {
	driver := gdw.ProtoDriver

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
func (gdw *NaiveDiffDriver) ApplyDiff(id, parent string, diff io.Reader) (size int64, err error) {
	driver := gdw.ProtoDriver

	// Mount the root filesystem so we can apply the diff/layer.
	layerRootFs, err := driver.Get(id, "")
	if err != nil {
		return
	}
	defer driver.Put(id)

	layerFs := layerRootFs
	options := &archive.TarOptions{IDMap: gdw.IDMap, BestEffortXattrs: gdw.BestEffortXattrs}
	start := time.Now().UTC()
	logrus.WithField("id", id).Debug("Start untar layer")
	if size, err = ApplyUncompressedLayer(layerFs, diff, options); err != nil {
		return
	}
	logrus.WithField("id", id).Debugf("Untar time: %vs", time.Now().UTC().Sub(start).Seconds())

	return
}

// DiffSize calculates the changes between the specified layer
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (gdw *NaiveDiffDriver) DiffSize(id, parent string) (size int64, err error) {
	driver := gdw.ProtoDriver

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
