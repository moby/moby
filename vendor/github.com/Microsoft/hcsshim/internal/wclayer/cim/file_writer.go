//go:build windows

package cim

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/internal/safefile"
	"github.com/Microsoft/hcsshim/internal/winapi"
)

// stdFileWriter writes the files of a layer to the layer folder instead of writing them inside the cim.
// For some files (like the Hive files or some UtilityVM files) it is necessary to write them as a normal file
// first, do some modifications on them (for example merging of hives or processing of UtilityVM files)
// and then write the modified versions into the cim. This writer is used for such files.
type stdFileWriter struct {
	activeFile *os.File
	// parent layer paths
	parentLayerPaths []string
	// path to the current layer
	path string
	// the open handle to the path directory
	root *os.File
}

func newStdFileWriter(root string, parentRoots []string) (sfw *stdFileWriter, err error) {
	sfw = &stdFileWriter{
		path:             root,
		parentLayerPaths: parentRoots,
	}
	sfw.root, err = safefile.OpenRoot(root)
	if err != nil {
		return
	}
	return
}

func (sfw *stdFileWriter) closeActiveFile() (err error) {
	if sfw.activeFile != nil {
		err = sfw.activeFile.Close()
		sfw.activeFile = nil
	}
	return
}

// Adds a new file or an alternate data stream to an existing file inside the layer directory.
func (sfw *stdFileWriter) Add(name string) error {
	if err := sfw.closeActiveFile(); err != nil {
		return err
	}

	// The directory of this file might be created inside the cim.
	// make sure we have the same parent directory chain here
	if err := safefile.MkdirAllRelative(filepath.Dir(name), sfw.root); err != nil {
		return fmt.Errorf("failed to create file %s: %w", name, err)
	}

	f, err := safefile.OpenRelative(
		name,
		sfw.root,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE|winio.WRITE_DAC|winio.WRITE_OWNER,
		syscall.FILE_SHARE_READ,
		winapi.FILE_CREATE,
		0,
	)
	if err != nil {
		return fmt.Errorf("error creating file %s: %w", name, err)
	}
	sfw.activeFile = f
	return nil
}

// Write writes data to the current file. The data must be in the format of a Win32
// backup stream.
func (sfw *stdFileWriter) Write(b []byte) (int, error) {
	return sfw.activeFile.Write(b)
}

// Close finishes the layer writing process and releases any resources.
func (sfw *stdFileWriter) Close(ctx context.Context) error {
	if err := sfw.closeActiveFile(); err != nil {
		return fmt.Errorf("failed to close active file %s : %w", sfw.activeFile.Name(), err)
	}
	return nil
}
