//go:build windows

package cim

import (
	"fmt"
	"io"
	"os"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/pkg/cimfs"
	"golang.org/x/sys/windows"
)

type pendingCimOp interface {
	apply(cw *cimfs.CimFsWriter) error
}

type pendingCimOpFunc func(cw *cimfs.CimFsWriter) error

func (f pendingCimOpFunc) apply(cw *cimfs.CimFsWriter) error {
	return f(cw)

}

// add op represents a pending operation of adding a new file inside the cim
type addOp struct {
	// path inside the cim at which the file should be added
	pathInCim string
	// host path where this file was temporarily written.
	hostPath string
	// other file metadata fields that were provided during the add call.
	fileInfo           *winio.FileBasicInfo
	securityDescriptor []byte
	extendedAttributes []byte
	reparseData        []byte
}

func (o *addOp) apply(cw *cimfs.CimFsWriter) error {
	f, err := os.Open(o.hostPath)
	if err != nil {
		return fmt.Errorf("open file %s: %w", o.hostPath, err)
	}
	defer f.Close()

	fs, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat file %s: %w", o.hostPath, err)
	}

	if err := cw.AddFile(o.pathInCim, o.fileInfo, fs.Size(), o.securityDescriptor, o.extendedAttributes, o.reparseData); err != nil {
		return fmt.Errorf("cim add file %s: %w", o.hostPath, err)
	}

	if o.fileInfo.FileAttributes != windows.FILE_ATTRIBUTE_DIRECTORY {
		written, err := io.Copy(cw, f)
		if err != nil {
			return fmt.Errorf("write file %s inside cim: %w", o.hostPath, err)
		} else if written != fs.Size() {
			return fmt.Errorf("short write to cim for file %s, expected %d bytes wrote %d", o.hostPath, fs.Size(), written)
		}
	}
	return nil
}

// linkOp represents a pending link file operation inside the cim
type linkOp struct {
	// old & new paths inside the cim where the link should be created
	oldPath string
	newPath string
}

func (o *linkOp) apply(cw *cimfs.CimFsWriter) error {
	return cw.AddLink(o.oldPath, o.newPath)
}
