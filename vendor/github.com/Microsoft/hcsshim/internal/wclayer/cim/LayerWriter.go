//go:build windows

package cim

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/oc"
	"github.com/Microsoft/hcsshim/internal/wclayer"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/Microsoft/hcsshim/pkg/cimfs"
	"go.opencensus.io/trace"
)

// A CimLayerWriter implements the wclayer.LayerWriter interface to allow writing container
// image layers in the cim format.
// A cim layer consist of cim files (which are usually stored in the `cim-layers` directory and
// some other files which are stored in the directory of that layer (i.e the `path` directory).
type CimLayerWriter struct {
	ctx context.Context
	s   *trace.Span
	// path to the layer (i.e layer's directory) as provided by the caller.
	// Even if a layer is stored as a cim in the cim directory, some files associated
	// with a layer are still stored in this path.
	path string
	// parent layer paths
	parentLayerPaths []string
	// Handle to the layer cim - writes to the cim file
	cimWriter *cimfs.CimFsWriter
	// Handle to the writer for writing files in the local filesystem
	stdFileWriter *stdFileWriter
	// reference to currently active writer either cimWriter or stdFileWriter
	activeWriter io.Writer
	// denotes if this layer has the UtilityVM directory
	hasUtilityVM bool
	// some files are written outside the cim during initial import (via stdFileWriter) because we need to
	// make some modifications to these files before writing them to the cim. The pendingOps slice
	// maintains a list of such delayed modifications to the layer cim. These modifications are applied at
	// the very end of layer import process.
	pendingOps []pendingCimOp
}

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

// Add adds a file to the layer with given metadata.
func (cw *CimLayerWriter) Add(name string, fileInfo *winio.FileBasicInfo, fileSize int64, securityDescriptor []byte, extendedAttributes []byte, reparseData []byte) error {
	if name == wclayer.UtilityVMPath {
		cw.hasUtilityVM = true
	}
	if isStdFile(name) {
		// create a pending op for this file
		cw.pendingOps = append(cw.pendingOps, &addOp{
			pathInCim:          name,
			hostPath:           filepath.Join(cw.path, name),
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
func (cw *CimLayerWriter) AddLink(name string, target string) error {
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
func (cw *CimLayerWriter) AddAlternateStream(name string, size uint64) error {
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

// Remove removes a file that was present in a parent layer from the layer.
func (cw *CimLayerWriter) Remove(name string) error {
	// set active write to nil so that we panic if layer tar is incorrectly formatted.
	cw.activeWriter = nil
	return cw.cimWriter.Unlink(name)
}

// Write writes data to the current file. The data must be in the format of a Win32
// backup stream.
func (cw *CimLayerWriter) Write(b []byte) (int, error) {
	return cw.activeWriter.Write(b)
}

// Close finishes the layer writing process and releases any resources.
func (cw *CimLayerWriter) Close(ctx context.Context) (retErr error) {
	if err := cw.stdFileWriter.Close(ctx); err != nil {
		return err
	}

	// cimWriter must be closed even if there are errors.
	defer func() {
		if err := cw.cimWriter.Close(); retErr == nil {
			retErr = err
		}
	}()

	// Find out the osversion of this layer, both base & non-base layers can have UtilityVM layer.
	processUtilityVM := false
	if cw.hasUtilityVM {
		uvmSoftwareHivePath := filepath.Join(cw.path, wclayer.UtilityVMPath, wclayer.RegFilesPath, "SOFTWARE")
		osvStr, err := getOsBuildNumberFromRegistry(uvmSoftwareHivePath)
		if err != nil {
			return fmt.Errorf("read os version string from UtilityVM SOFTWARE hive: %w", err)
		}

		osv, err := strconv.ParseUint(osvStr, 10, 16)
		if err != nil {
			return fmt.Errorf("parse os version string (%s): %w", osvStr, err)
		}

		// write this version to a file for future reference by the shim process
		if err = wclayer.WriteLayerUvmBuildFile(cw.path, uint16(osv)); err != nil {
			return fmt.Errorf("write uvm build version: %w", err)
		}

		// CIMFS for hyperV isolated is only supported after 20348, processing UtilityVM layer on 2048
		// & lower will cause failures since those images won't have CIMFS specific UVM files (mostly
		// BCD entries required for CIMFS)
		processUtilityVM = (osv > osversion.LTSC2022)
		log.G(ctx).Debugf("import image os version %d, processing UtilityVM layer: %t\n", osv, processUtilityVM)
	}

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

func NewCimLayerWriter(ctx context.Context, path string, parentLayerPaths []string) (_ *CimLayerWriter, err error) {
	if !cimfs.IsCimFSSupported() {
		return nil, fmt.Errorf("CimFs not supported on this build")
	}

	ctx, span := trace.StartSpan(ctx, "hcsshim::NewCimLayerWriter")
	defer func() {
		if err != nil {
			oc.SetSpanStatus(span, err)
			span.End()
		}
	}()
	span.AddAttributes(
		trace.StringAttribute("path", path),
		trace.StringAttribute("parentLayerPaths", strings.Join(parentLayerPaths, ", ")))

	parentCim := ""
	cimDirPath := GetCimDirFromLayer(path)
	if _, err = os.Stat(cimDirPath); os.IsNotExist(err) {
		// create cim directory
		if err = os.Mkdir(cimDirPath, 0755); err != nil {
			return nil, fmt.Errorf("failed while creating cim layers directory: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("unable to access cim layers directory: %w", err)

	}

	if len(parentLayerPaths) > 0 {
		parentCim = GetCimNameFromLayer(parentLayerPaths[0])
	}

	cim, err := cimfs.Create(cimDirPath, parentCim, GetCimNameFromLayer(path))
	if err != nil {
		return nil, fmt.Errorf("error in creating a new cim: %w", err)
	}

	sfw, err := newStdFileWriter(path, parentLayerPaths)
	if err != nil {
		return nil, fmt.Errorf("error in creating new standard file writer: %w", err)
	}
	return &CimLayerWriter{
		ctx:              ctx,
		s:                span,
		path:             path,
		parentLayerPaths: parentLayerPaths,
		cimWriter:        cim,
		stdFileWriter:    sfw,
	}, nil
}

// DestroyCimLayer destroys a cim layer i.e it removes all the cimfs files for the given layer as well as
// all of the other files that are stored in the layer directory (at path `layerPath`).
// If this is not a cimfs layer (i.e a cim file for the given layer does not exist) then nothing is done.
func DestroyCimLayer(ctx context.Context, layerPath string) error {
	cimPath := GetCimPathFromLayer(layerPath)

	// verify that such a cim exists first, sometimes containerd tries to call
	// this with the root snapshot directory as the layer path. We don't want to
	// destroy everything inside the snapshots directory.
	if _, err := os.Stat(cimPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return cimfs.DestroyCim(ctx, cimPath)
}
