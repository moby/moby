package hcsshim

import (
	"io/ioutil"
	"os"
	"runtime"

	"github.com/Microsoft/go-winio"
	"github.com/Sirupsen/logrus"
)

// ImportLayer will take the contents of the folder at importFolderPath and import
// that into a layer with the id layerId.  Note that in order to correctly populate
// the layer and interperet the transport format, all parent layers must already
// be present on the system at the paths provided in parentLayerPaths.
func ImportLayer(info DriverInfo, layerId string, importFolderPath string, parentLayerPaths []string) error {
	title := "hcsshim::ImportLayer "
	logrus.Debugf(title+"flavour %d layerId %s folder %s", info.Flavour, layerId, importFolderPath)

	// Generate layer descriptors
	layers, err := layerPathsToDescriptors(parentLayerPaths)
	if err != nil {
		return err
	}

	// Convert info to API calling convention
	infop, err := convertDriverInfo(info)
	if err != nil {
		logrus.Error(err)
		return err
	}

	err = importLayer(&infop, layerId, importFolderPath, layers)
	if err != nil {
		err = makeErrorf(err, title, "layerId=%s flavour=%d folder=%s", layerId, info.Flavour, importFolderPath)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+"succeeded flavour=%d layerId=%s folder=%s", info.Flavour, layerId, importFolderPath)
	return nil
}

type LayerWriter interface {
	Add(name string, fileInfo *winio.FileBasicInfo) error
	Remove(name string) error
	Write(b []byte) (int, error)
	Close() error
}

// FilterLayerWriter provides an interface to write the contents of a layer to the file system.
type FilterLayerWriter struct {
	context uintptr
}

// Add adds a file or directory to the layer. The file's parent directory must have already been added.
//
// name contains the file's relative path. fileInfo contains file times and file attributes; the rest
// of the file metadata and the file data must be written as a Win32 backup stream to the Write() method.
// winio.BackupStreamWriter can be used to facilitate this.
func (w *FilterLayerWriter) Add(name string, fileInfo *winio.FileBasicInfo) error {
	if name[0] != '\\' {
		name = `\` + name
	}
	err := importLayerNext(w.context, name, fileInfo)
	if err != nil {
		return makeError(err, "ImportLayerNext", "")
	}
	return nil
}

// Remove removes a file from the layer. The file must have been present in the parent layer.
//
// name contains the file's relative path.
func (w *FilterLayerWriter) Remove(name string) error {
	if name[0] != '\\' {
		name = `\` + name
	}
	err := importLayerNext(w.context, name, nil)
	if err != nil {
		return makeError(err, "ImportLayerNext", "")
	}
	return nil
}

// Write writes more backup stream data to the current file.
func (w *FilterLayerWriter) Write(b []byte) (int, error) {
	err := importLayerWrite(w.context, b)
	if err != nil {
		err = makeError(err, "ImportLayerWrite", "")
		return 0, err
	}
	return len(b), err
}

// Close completes the layer write operation. The error must be checked to ensure that the
// operation was successful.
func (w *FilterLayerWriter) Close() (err error) {
	if w.context != 0 {
		err = importLayerEnd(w.context)
		if err != nil {
			err = makeError(err, "ImportLayerEnd", "")
		}
		w.context = 0
	}
	return
}

type legacyLayerWriterWrapper struct {
	*LegacyLayerWriter
	info             DriverInfo
	layerId          string
	parentLayerPaths []string
}

func (r *legacyLayerWriterWrapper) Close() error {
	err := r.LegacyLayerWriter.Close()
	if err == nil {
		err = ImportLayer(r.info, r.layerId, r.root, r.parentLayerPaths)
	}
	os.RemoveAll(r.root)
	return err
}

// NewLayerWriter returns a new layer writer for creating a layer on disk.
func NewLayerWriter(info DriverInfo, layerId string, parentLayerPaths []string) (LayerWriter, error) {
	if procImportLayerBegin.Find() != nil {
		// The new layer reader is not available on this Windows build. Fall back to the
		// legacy export code path.
		path, err := ioutil.TempDir("", "hcs")
		if err != nil {
			return nil, err
		}
		return &legacyLayerWriterWrapper{NewLegacyLayerWriter(path), info, layerId, parentLayerPaths}, nil
	}
	layers, err := layerPathsToDescriptors(parentLayerPaths)
	if err != nil {
		return nil, err
	}

	infop, err := convertDriverInfo(info)
	if err != nil {
		return nil, err
	}

	w := &FilterLayerWriter{}
	err = importLayerBegin(&infop, layerId, layers, &w.context)
	if err != nil {
		return nil, makeError(err, "ImportLayerStart", "")
	}
	runtime.SetFinalizer(w, func(w *FilterLayerWriter) { w.Close() })
	return w, nil
}
