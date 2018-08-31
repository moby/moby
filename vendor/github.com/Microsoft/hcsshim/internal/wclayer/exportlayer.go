package wclayer

import (
	"io"
	"io/ioutil"
	"os"
	"syscall"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/internal/hcserror"
	"github.com/Microsoft/hcsshim/internal/interop"
	"github.com/sirupsen/logrus"
)

// ExportLayer will create a folder at exportFolderPath and fill that folder with
// the transport format version of the layer identified by layerId. This transport
// format includes any metadata required for later importing the layer (using
// ImportLayer), and requires the full list of parent layer paths in order to
// perform the export.
func ExportLayer(path string, exportFolderPath string, parentLayerPaths []string) error {
	title := "hcsshim::ExportLayer "
	logrus.Debugf(title+"path %s folder %s", path, exportFolderPath)

	// Generate layer descriptors
	layers, err := layerPathsToDescriptors(parentLayerPaths)
	if err != nil {
		return err
	}

	err = exportLayer(&stdDriverInfo, path, exportFolderPath, layers)
	if err != nil {
		err = hcserror.Errorf(err, title, "path=%s folder=%s", path, exportFolderPath)
		logrus.Error(err)
		return err
	}

	logrus.Debugf(title+"succeeded path=%s folder=%s", path, exportFolderPath)
	return nil
}

type LayerReader interface {
	Next() (string, int64, *winio.FileBasicInfo, error)
	Read(b []byte) (int, error)
	Close() error
}

// FilterLayerReader provides an interface for extracting the contents of an on-disk layer.
type FilterLayerReader struct {
	context uintptr
}

// Next reads the next available file from a layer, ensuring that parent directories are always read
// before child files and directories.
//
// Next returns the file's relative path, size, and basic file metadata. Read() should be used to
// extract a Win32 backup stream with the remainder of the metadata and the data.
func (r *FilterLayerReader) Next() (string, int64, *winio.FileBasicInfo, error) {
	var fileNamep *uint16
	fileInfo := &winio.FileBasicInfo{}
	var deleted uint32
	var fileSize int64
	err := exportLayerNext(r.context, &fileNamep, fileInfo, &fileSize, &deleted)
	if err != nil {
		if err == syscall.ERROR_NO_MORE_FILES {
			err = io.EOF
		} else {
			err = hcserror.New(err, "ExportLayerNext", "")
		}
		return "", 0, nil, err
	}
	fileName := interop.ConvertAndFreeCoTaskMemString(fileNamep)
	if deleted != 0 {
		fileInfo = nil
	}
	if fileName[0] == '\\' {
		fileName = fileName[1:]
	}
	return fileName, fileSize, fileInfo, nil
}

// Read reads from the current file's Win32 backup stream.
func (r *FilterLayerReader) Read(b []byte) (int, error) {
	var bytesRead uint32
	err := exportLayerRead(r.context, b, &bytesRead)
	if err != nil {
		return 0, hcserror.New(err, "ExportLayerRead", "")
	}
	if bytesRead == 0 {
		return 0, io.EOF
	}
	return int(bytesRead), nil
}

// Close frees resources associated with the layer reader. It will return an
// error if there was an error while reading the layer or of the layer was not
// completely read.
func (r *FilterLayerReader) Close() (err error) {
	if r.context != 0 {
		err = exportLayerEnd(r.context)
		if err != nil {
			err = hcserror.New(err, "ExportLayerEnd", "")
		}
		r.context = 0
	}
	return
}

// NewLayerReader returns a new layer reader for reading the contents of an on-disk layer.
// The caller must have taken the SeBackupPrivilege privilege
// to call this and any methods on the resulting LayerReader.
func NewLayerReader(path string, parentLayerPaths []string) (LayerReader, error) {
	if procExportLayerBegin.Find() != nil {
		// The new layer reader is not available on this Windows build. Fall back to the
		// legacy export code path.
		exportPath, err := ioutil.TempDir("", "hcs")
		if err != nil {
			return nil, err
		}
		err = ExportLayer(path, exportPath, parentLayerPaths)
		if err != nil {
			os.RemoveAll(exportPath)
			return nil, err
		}
		return &legacyLayerReaderWrapper{newLegacyLayerReader(exportPath)}, nil
	}

	layers, err := layerPathsToDescriptors(parentLayerPaths)
	if err != nil {
		return nil, err
	}
	r := &FilterLayerReader{}
	err = exportLayerBegin(&stdDriverInfo, path, layers, &r.context)
	if err != nil {
		return nil, hcserror.New(err, "ExportLayerBegin", "")
	}
	return r, err
}

type legacyLayerReaderWrapper struct {
	*legacyLayerReader
}

func (r *legacyLayerReaderWrapper) Close() error {
	err := r.legacyLayerReader.Close()
	os.RemoveAll(r.root)
	return err
}
