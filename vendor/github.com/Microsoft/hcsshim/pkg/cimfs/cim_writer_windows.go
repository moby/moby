//go:build windows
// +build windows

package cimfs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/winapi"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

// CimFsWriter represents a writer to a single CimFS filesystem instance. On disk, the
// image is composed of a filesystem file and several object ID and region files.
// Note: The CimFsWriter isn't thread safe!
type CimFsWriter struct {
	// name of this cim. Usually a <name>.cim file will be created to represent this cim.
	name string
	// handle is the CIMFS_IMAGE_HANDLE that must be passed when calling CIMFS APIs.
	handle winapi.FsHandle
	// name of the active file i.e the file to which we are currently writing.
	activeName string
	// stream to currently active file.
	activeStream winapi.StreamHandle
	// amount of bytes that can be written to the activeStream.
	activeLeft uint64
}

// Create creates a new cim image. The CimFsWriter returned can then be used to do
// operations on this cim.
func Create(imagePath string, oldFSName string, newFSName string) (_ *CimFsWriter, err error) {
	var oldNameBytes *uint16
	// CimCreateImage API call has different behavior if the value of oldNameBytes / newNameBytes
	// is empty than if it is nil. So we have to convert those strings into *uint16 here.
	fsName := oldFSName
	if oldFSName != "" {
		oldNameBytes, err = windows.UTF16PtrFromString(oldFSName)
		if err != nil {
			return nil, err
		}
	}
	var newNameBytes *uint16
	if newFSName != "" {
		fsName = newFSName
		newNameBytes, err = windows.UTF16PtrFromString(newFSName)
		if err != nil {
			return nil, err
		}
	}
	var handle winapi.FsHandle
	if err := winapi.CimCreateImage(imagePath, oldNameBytes, newNameBytes, &handle); err != nil {
		return nil, fmt.Errorf("failed to create cim image at path %s, oldName: %s, newName: %s: %w", imagePath, oldFSName, newFSName, err)
	}
	return &CimFsWriter{handle: handle, name: filepath.Join(imagePath, fsName)}, nil
}

// CreateAlternateStream creates alternate stream of given size at the given path inside the cim. This will
// replace the current active stream. Always, finish writing current active stream and then create an
// alternate stream.
func (c *CimFsWriter) CreateAlternateStream(path string, size uint64) (err error) {
	err = c.closeStream()
	if err != nil {
		return err
	}
	err = winapi.CimCreateAlternateStream(c.handle, path, size, &c.activeStream)
	if err != nil {
		return fmt.Errorf("failed to create alternate stream for path %s: %w", path, err)
	}
	c.activeName = path
	return nil
}

// closes the currently active stream.
func (c *CimFsWriter) closeStream() error {
	if c.activeStream == 0 {
		return nil
	}
	err := winapi.CimCloseStream(c.activeStream)
	if err == nil && c.activeLeft > 0 {
		// Validate here because CimCloseStream does not and this improves error
		// reporting. Otherwise the error will occur in the context of
		// cimWriteStream.
		err = fmt.Errorf("incomplete write, %d bytes left in the stream %s", c.activeLeft, c.activeName)
	}
	if err != nil {
		err = &PathError{Cim: c.name, Op: "closeStream", Path: c.activeName, Err: err}
	}
	c.activeLeft = 0
	c.activeStream = 0
	c.activeName = ""
	return err
}

// AddFile adds a new file to the image. The file is added at the specified path. After
// calling this function, the file is set as the active stream for the image, so data can
// be written by calling `Write`.
func (c *CimFsWriter) AddFile(path string, info *winio.FileBasicInfo, fileSize int64, securityDescriptor []byte, extendedAttributes []byte, reparseData []byte) error {
	err := c.closeStream()
	if err != nil {
		return err
	}
	fileMetadata := &winapi.CimFsFileMetadata{
		Attributes:     info.FileAttributes,
		FileSize:       fileSize,
		CreationTime:   info.CreationTime,
		LastWriteTime:  info.LastWriteTime,
		ChangeTime:     info.ChangeTime,
		LastAccessTime: info.LastAccessTime,
	}
	if len(securityDescriptor) == 0 {
		// Passing an empty security descriptor creates a CIM in a weird state.
		// Pass the NULL DACL.
		securityDescriptor = nullSd
	}
	fileMetadata.SecurityDescriptorBuffer = unsafe.Pointer(&securityDescriptor[0])
	fileMetadata.SecurityDescriptorSize = uint32(len(securityDescriptor))
	if len(reparseData) > 0 {
		fileMetadata.ReparseDataBuffer = unsafe.Pointer(&reparseData[0])
		fileMetadata.ReparseDataSize = uint32(len(reparseData))
	}
	if len(extendedAttributes) > 0 {
		fileMetadata.ExtendedAttributes = unsafe.Pointer(&extendedAttributes[0])
		fileMetadata.EACount = uint32(len(extendedAttributes))
	}
	// remove the trailing `\` if present, otherwise it trips off the cim writer
	path = strings.TrimSuffix(path, "\\")
	err = winapi.CimCreateFile(c.handle, path, fileMetadata, &c.activeStream)
	if err != nil {
		return &PathError{Cim: c.name, Op: "addFile", Path: path, Err: err}
	}
	c.activeName = path
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY) == 0 {
		c.activeLeft = uint64(fileSize)
	}
	return nil
}

// Write writes bytes to the active stream.
func (c *CimFsWriter) Write(p []byte) (int, error) {
	if c.activeStream == 0 {
		return 0, fmt.Errorf("no active stream")
	}
	if uint64(len(p)) > c.activeLeft {
		return 0, &PathError{Cim: c.name, Op: "write", Path: c.activeName, Err: fmt.Errorf("wrote too much")}
	}
	err := winapi.CimWriteStream(c.activeStream, uintptr(unsafe.Pointer(&p[0])), uint32(len(p)))
	if err != nil {
		err = &PathError{Cim: c.name, Op: "write", Path: c.activeName, Err: err}
		return 0, err
	}
	c.activeLeft -= uint64(len(p))
	return len(p), nil
}

// AddLink adds a hard link from `oldPath` to `newPath` in the image.
func (c *CimFsWriter) AddLink(oldPath string, newPath string) error {
	err := c.closeStream()
	if err != nil {
		return err
	}
	err = winapi.CimCreateHardLink(c.handle, newPath, oldPath)
	if err != nil {
		err = &LinkError{Cim: c.name, Op: "addLink", Old: oldPath, New: newPath, Err: err}
	}
	return err
}

// Unlink deletes the file at `path` from the image.
func (c *CimFsWriter) Unlink(path string) error {
	err := c.closeStream()
	if err != nil {
		return err
	}
	//TODO(ambarve): CimDeletePath currently returns an error if the file isn't found but we ideally want
	// to put a tombstone at that path so that when cims are merged it removes that file from the lower
	// layer
	err = winapi.CimDeletePath(c.handle, path)
	if err != nil && !os.IsNotExist(err) {
		err = &PathError{Cim: c.name, Op: "unlink", Path: path, Err: err}
		return err
	}
	return nil
}

func (c *CimFsWriter) commit() error {
	err := c.closeStream()
	if err != nil {
		return err
	}
	err = winapi.CimCommitImage(c.handle)
	if err != nil {
		err = &OpError{Cim: c.name, Op: "commit", Err: err}
	}
	return err
}

// Close closes the CimFS filesystem.
func (c *CimFsWriter) Close() error {
	if c.handle == 0 {
		return fmt.Errorf("invalid writer")
	}
	if err := c.commit(); err != nil {
		return &OpError{Cim: c.name, Op: "commit", Err: err}
	}
	if err := winapi.CimCloseImage(c.handle); err != nil {
		return &OpError{Cim: c.name, Op: "close", Err: err}
	}
	c.handle = 0
	return nil
}

// DestroyCim finds out the region files, object files of this cim and then delete
// the region files, object files and the <layer-id>.cim file itself.
func DestroyCim(ctx context.Context, cimPath string) (retErr error) {
	regionFilePaths, err := getRegionFilePaths(ctx, cimPath)
	if err != nil {
		log.G(ctx).WithError(err).Warnf("get region files for cim %s", cimPath)
		if retErr == nil { //nolint:govet // nilness: consistency with below
			retErr = err
		}
	}
	objectFilePaths, err := getObjectIDFilePaths(ctx, cimPath)
	if err != nil {
		log.G(ctx).WithError(err).Warnf("get objectid file for cim %s", cimPath)
		if retErr == nil {
			retErr = err
		}
	}

	log.G(ctx).WithFields(logrus.Fields{
		"cimPath":     cimPath,
		"regionFiles": regionFilePaths,
		"objectFiles": objectFilePaths,
	}).Debug("destroy cim")

	for _, regFilePath := range regionFilePaths {
		if err := os.Remove(regFilePath); err != nil {
			log.G(ctx).WithError(err).Warnf("remove file %s", regFilePath)
			if retErr == nil {
				retErr = err
			}
		}
	}

	for _, objFilePath := range objectFilePaths {
		if err := os.Remove(objFilePath); err != nil {
			log.G(ctx).WithError(err).Warnf("remove file %s", objFilePath)
			if retErr == nil {
				retErr = err
			}
		}
	}

	if err := os.Remove(cimPath); err != nil {
		log.G(ctx).WithError(err).Warnf("remove file %s", cimPath)
		if retErr == nil {
			retErr = err
		}
	}
	return retErr
}

// GetCimUsage returns the total disk usage in bytes by the cim at path `cimPath`.
func GetCimUsage(ctx context.Context, cimPath string) (uint64, error) {
	regionFilePaths, err := getRegionFilePaths(ctx, cimPath)
	if err != nil {
		return 0, fmt.Errorf("get region file paths for cim %s: %w", cimPath, err)
	}
	objectFilePaths, err := getObjectIDFilePaths(ctx, cimPath)
	if err != nil {
		return 0, fmt.Errorf("get objectid file for cim %s: %w", cimPath, err)
	}

	var totalUsage uint64
	for _, f := range append(regionFilePaths, objectFilePaths...) {
		fi, err := os.Stat(f)
		if err != nil {
			return 0, fmt.Errorf("stat file %s: %w", f, err)
		}
		totalUsage += uint64(fi.Size())
	}
	return totalUsage, nil
}
