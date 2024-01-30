//go:build windows
// +build windows

package cimfs

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/pkg/cimfs/format"
)

var (
	// Equivalent to SDDL of "D:NO_ACCESS_CONTROL".
	nullSd = []byte{1, 0, 4, 128, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
)

type OpError struct {
	Cim string
	Op  string
	Err error
}

func (e *OpError) Error() string {
	s := "cim " + e.Op + " " + e.Cim
	s += ": " + e.Err.Error()
	return s
}

// PathError is the error type returned by most functions in this package.
type PathError struct {
	Cim  string
	Op   string
	Path string
	Err  error
}

func (e *PathError) Error() string {
	s := "cim " + e.Op + " " + e.Cim
	s += ":" + e.Path
	s += ": " + e.Err.Error()
	return s
}

type LinkError struct {
	Cim string
	Op  string
	Old string
	New string
	Err error
}

func (e *LinkError) Error() string {
	return "cim " + e.Op + " " + e.Old + " " + e.New + ": " + e.Err.Error()
}

func validateHeader(h *format.CommonHeader) error {
	if !bytes.Equal(h.Magic[:], format.MagicValue[:]) {
		return fmt.Errorf("not a cim file")
	}
	if h.Version.Major > format.CurrentVersion.Major || h.Version.Major < format.MinSupportedVersion.Major {
		return fmt.Errorf("unsupported cim version. cim version %v must be between %v & %v", h.Version, format.MinSupportedVersion, format.CurrentVersion)
	}
	return nil
}

func readFilesystemHeader(f *os.File) (format.FilesystemHeader, error) {
	var fsh format.FilesystemHeader

	if err := binary.Read(f, binary.LittleEndian, &fsh); err != nil {
		return fsh, fmt.Errorf("reading filesystem header: %w", err)
	}

	if err := validateHeader(&fsh.Common); err != nil {
		return fsh, fmt.Errorf("validating filesystem header: %w", err)
	}
	return fsh, nil
}

// Returns the paths of all the objectID files associated with the cim at `cimPath`.
func getObjectIDFilePaths(ctx context.Context, cimPath string) ([]string, error) {
	f, err := os.Open(cimPath)
	if err != nil {
		return []string{}, fmt.Errorf("open cim file %s: %w", cimPath, err)
	}
	defer f.Close()

	fsh, err := readFilesystemHeader(f)
	if err != nil {
		return []string{}, fmt.Errorf("readingp cim header: %w", err)
	}

	paths := []string{}
	for i := 0; i < int(fsh.Regions.Count); i++ {
		path := filepath.Join(filepath.Dir(cimPath), fmt.Sprintf("%s_%v_%d", format.ObjectIDFileName, fsh.Regions.ID, i))
		if _, err := os.Stat(path); err == nil {
			paths = append(paths, path)
		} else {
			log.G(ctx).WithError(err).Warnf("stat for object file %s", path)
		}

	}
	return paths, nil
}

// Returns the paths of all the region files associated with the cim at `cimPath`.
func getRegionFilePaths(ctx context.Context, cimPath string) ([]string, error) {
	f, err := os.Open(cimPath)
	if err != nil {
		return []string{}, fmt.Errorf("open cim file %s: %w", cimPath, err)
	}
	defer f.Close()

	fsh, err := readFilesystemHeader(f)
	if err != nil {
		return []string{}, fmt.Errorf("reading cim header: %w", err)
	}

	paths := []string{}
	for i := 0; i < int(fsh.Regions.Count); i++ {
		path := filepath.Join(filepath.Dir(cimPath), fmt.Sprintf("%s_%v_%d", format.RegionFileName, fsh.Regions.ID, i))
		if _, err := os.Stat(path); err == nil {
			paths = append(paths, path)
		} else {
			log.G(ctx).WithError(err).Warnf("stat for region file %s", path)
		}
	}
	return paths, nil
}
