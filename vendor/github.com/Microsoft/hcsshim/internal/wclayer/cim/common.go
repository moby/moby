//go:build windows

package cim

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/internal/wclayer"
	"github.com/Microsoft/hcsshim/pkg/cimfs"
)

var (
	ErrBlockCIMWriterNotSupported = fmt.Errorf("writing block device CIM isn't supported")
	ErrBlockCIMParentTypeMismatch = fmt.Errorf("parent layer block CIM type doesn't match with extraction layer")
	ErrBlockCIMIntegrityMismatch  = fmt.Errorf("verified CIMs can not be mixed with non verified CIMs")
)

type hive struct {
	name  string
	base  string
	delta string
}

var (
	hives = []hive{
		{"SYSTEM", "SYSTEM_BASE", "SYSTEM_DELTA"},
		{"SOFTWARE", "SOFTWARE_BASE", "SOFTWARE_DELTA"},
		{"SAM", "SAM_BASE", "SAM_DELTA"},
		{"SECURITY", "SECURITY_BASE", "SECURITY_DELTA"},
		{"DEFAULT", "DEFAULTUSER_BASE", "DEFAULTUSER_DELTA"},
	}
)

// CIMLayerWriter is an interface that supports writing a new container image layer to the
// CIM format
type CIMLayerWriter interface {
	// Add adds a file to the layer with given metadata.
	Add(string, *winio.FileBasicInfo, int64, []byte, []byte, []byte) error
	// AddLink adds a hard link to the layer. The target must already have been added.
	AddLink(string, string) error
	// AddAlternateStream adds an alternate stream to a file
	AddAlternateStream(string, uint64) error
	// Remove removes a file that was present in a parent layer from the layer.
	Remove(string) error
	// Write writes data to the current file. The data must be in the format of a Win32
	// backup stream.
	Write([]byte) (int, error)
	// Close finishes the layer writing process and releases any resources.
	Close(context.Context) error
}

func isDeltaOrBaseHive(path string) bool {
	for _, hv := range hives {
		if strings.EqualFold(path, filepath.Join(wclayer.HivesPath, hv.delta)) ||
			strings.EqualFold(path, filepath.Join(wclayer.RegFilesPath, hv.name)) {
			return true
		}
	}
	return false
}

// checks if this particular file should be written with a stdFileWriter instead of
// using the cimWriter.
func isStdFile(path string) bool {
	return (isDeltaOrBaseHive(path) ||
		path == filepath.Join(wclayer.UtilityVMPath, wclayer.RegFilesPath, "SYSTEM") ||
		path == filepath.Join(wclayer.UtilityVMPath, wclayer.RegFilesPath, "SOFTWARE") ||
		path == wclayer.BcdFilePath || path == wclayer.BootMgrFilePath)
}

// cimLayerWriter is a base struct that is further extended by forked cim writer & blocked
// cim writer to provide full functionality of writing layers.
type cimLayerWriter struct {
	ctx context.Context
	// Handle to the layer cim - writes to the cim file
	cimWriter *cimfs.CimFsWriter
	// Handle to the writer for writing files in the local filesystem
	stdFileWriter *stdFileWriter
	// reference to currently active writer either cimWriter or stdFileWriter
	activeWriter io.Writer
	// denotes if this layer has the UtilityVM directory
	hasUtilityVM bool
	// path to the layer (i.e layer's directory) as provided by the caller.
	// Even if a layer is stored as a cim in the cim directory, some files associated
	// with a layer are still stored in this path.
	layerPath string
	// parent layer paths
	parentLayerPaths []string
	// some files are written outside the cim during initial import (via stdFileWriter) because we need to
	// make some modifications to these files before writing them to the cim. The pendingOps slice
	// maintains a list of such delayed modifications to the layer cim. These modifications are applied at
	// the very end of layer import process.
	pendingOps []pendingCimOp
}

// Add adds a file to the layer with given metadata.
func (cw *cimLayerWriter) Add(name string, fileInfo *winio.FileBasicInfo, fileSize int64, securityDescriptor []byte, extendedAttributes []byte, reparseData []byte) error {
	if name == wclayer.UtilityVMPath {
		cw.hasUtilityVM = true
	}
	if isStdFile(name) {
		// create a pending op for this file
		cw.pendingOps = append(cw.pendingOps, &addOp{
			pathInCim:          name,
			hostPath:           filepath.Join(cw.layerPath, name),
			fileInfo:           fileInfo,
			securityDescriptor: securityDescriptor,
			extendedAttributes: extendedAttributes,
			reparseData:        reparseData,
		})
		if err := cw.stdFileWriter.Add(name); err != nil {
			return err
		}
		cw.activeWriter = cw.stdFileWriter
	} else {
		if err := cw.cimWriter.AddFile(name, fileInfo, fileSize, securityDescriptor, extendedAttributes, reparseData); err != nil {
			return err
		}
		cw.activeWriter = cw.cimWriter
	}
	return nil
}

// AddLink adds a hard link to the layer. The target must already have been added.
func (cw *cimLayerWriter) AddLink(name string, target string) error {
	// set active write to nil so that we panic if layer tar is incorrectly formatted.
	cw.activeWriter = nil
	if isStdFile(target) {
		// If this is a link to a std file it will have to be added later once the
		// std file is written to the CIM. Create a pending op for this
		cw.pendingOps = append(cw.pendingOps, &linkOp{
			oldPath: target,
			newPath: name,
		})
		return nil
	} else if isStdFile(name) {
		// None of the predefined std files are links. If they show up as links this is unexpected
		// behavior. Error out.
		return fmt.Errorf("unexpected link %s in layer", name)
	} else {
		return cw.cimWriter.AddLink(target, name)
	}
}

// AddAlternateStream creates another alternate stream at the given
// path. Any writes made after this call will go to that stream.
func (cw *cimLayerWriter) AddAlternateStream(name string, size uint64) error {
	if isStdFile(name) {
		// As of now there is no known case of std file having multiple data streams.
		// If such a file is encountered our assumptions are wrong. Error out.
		return fmt.Errorf("unexpected alternate stream %s in layer", name)
	}

	if err := cw.cimWriter.CreateAlternateStream(name, size); err != nil {
		return err
	}
	cw.activeWriter = cw.cimWriter
	return nil
}

// Write writes data to the current file. The data must be in the format of a Win32
// backup stream.
func (cw *cimLayerWriter) Write(b []byte) (int, error) {
	return cw.activeWriter.Write(b)
}

// Close finishes the layer writing process and releases any resources.
func (cw *cimLayerWriter) Close(ctx context.Context) (retErr error) {
	if err := cw.stdFileWriter.Close(ctx); err != nil {
		return err
	}

	// cimWriter must be closed even if there are errors.
	defer func() {
		if err := cw.cimWriter.Close(); retErr == nil {
			retErr = err
		}
	}()

	// We don't support running UtilityVM with CIM layers yet.
	processUtilityVM := false

	if len(cw.parentLayerPaths) == 0 {
		if err := cw.processBaseLayer(ctx, processUtilityVM); err != nil {
			return fmt.Errorf("process base layer: %w", err)
		}
	} else {
		if err := cw.processNonBaseLayer(ctx, processUtilityVM); err != nil {
			return fmt.Errorf("process non base layer: %w", err)
		}
	}

	for _, op := range cw.pendingOps {
		if err := op.apply(cw.cimWriter); err != nil {
			return fmt.Errorf("apply pending operations: %w", err)
		}
	}
	return nil
}
