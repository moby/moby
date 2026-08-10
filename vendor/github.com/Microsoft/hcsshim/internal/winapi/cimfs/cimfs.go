//go:build windows

package cimfs

import (
	"sync"

	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"

	"github.com/Microsoft/hcsshim/internal/winapi/types"
)

// Type aliases

type GUID = guid.GUID
type FsHandle = types.FsHandle
type StreamHandle = types.StreamHandle
type FileMetadata = types.CimFsFileMetadata
type ImagePath = types.CimFsImagePath

//sys CimMountImage(imagePath string, fsName string, flags uint32, volumeID *GUID) (hr error) = cimfs.CimMountImage?
//sys CimDismountImage(volumeID *GUID) (hr error) = cimfs.CimDismountImage?

//sys CimCreateImage(imagePath string, oldFSName *uint16, newFSName *uint16, cimFSHandle *FsHandle) (hr error) = cimfs.CimCreateImage?
//sys CimCreateImage2(imagePath string, flags uint32, oldFSName *uint16, newFSName *uint16, cimFSHandle *FsHandle) (hr error) = cimfs.CimCreateImage2?
//sys CimCloseImage(cimFSHandle FsHandle) = cimfs.CimCloseImage?
//sys CimCommitImage(cimFSHandle FsHandle) (hr error) = cimfs.CimCommitImage?

//sys CimCreateFile(cimFSHandle FsHandle, path string, file *FileMetadata, cimStreamHandle *StreamHandle) (hr error) = cimfs.CimCreateFile?
//sys CimCloseStream(cimStreamHandle StreamHandle) (hr error) = cimfs.CimCloseStream?
//sys CimWriteStream(cimStreamHandle StreamHandle, buffer uintptr, bufferSize uint32) (hr error) = cimfs.CimWriteStream?
//sys CimDeletePath(cimFSHandle FsHandle, path string) (hr error) = cimfs.CimDeletePath?
//sys CimCreateHardLink(cimFSHandle FsHandle, newPath string, oldPath string) (hr error) = cimfs.CimCreateHardLink?
//sys CimCreateAlternateStream(cimFSHandle FsHandle, path string, size uint64, cimStreamHandle *StreamHandle) (hr error) = cimfs.CimCreateAlternateStream?
//sys CimAddFsToMergedImage(cimFSHandle FsHandle, path string) (hr error) = cimfs.CimAddFsToMergedImage?
//sys CimAddFsToMergedImage2(cimFSHandle FsHandle, path string, flags uint32) (hr error) = cimfs.CimAddFsToMergedImage2?
//sys CimMergeMountImage(numCimPaths uint32, backingImagePaths *ImagePath, flags uint32, volumeID *GUID) (hr error) = cimfs.CimMergeMountImage?
//sys CimTombstoneFile(cimFSHandle FsHandle, path string) (hr error) = cimfs.CimTombstoneFile?
//sys CimCreateMergeLink(cimFSHandle FsHandle, newPath string, oldPath string) (hr error) = cimfs.CimCreateMergeLink?
//sys CimSealImage(blockCimPath string, hashSize *uint64, fixedHeaderSize *uint64, hash *byte) (hr error) = cimfs.CimSealImage?
//sys CimGetVerificationInformation(blockCimPath string, isSealed *uint32, hashSize *uint64, signatureSize *uint64, fixedHeaderSize *uint64, hash *byte, signature *byte) (hr error) = cimfs.CimGetVerificationInformation?
//sys CimMountVerifiedImage(imagePath string, fsName string, flags uint32, volumeID *GUID, hashSize uint16, hash *byte) (hr error) = cimfs.CimMountVerifiedImage?
//sys CimMergeMountVerifiedImage(numCimPaths uint32, backingImagePaths *ImagePath, flags uint32, volumeID *GUID, hashSize uint16, hash *byte) (hr error) = cimfs.CimMergeMountVerifiedImage?

var load = sync.OnceValue(func() error {
	if err := modcimfs.Load(); err != nil {
		return err
	}
	var buf [windows.MAX_PATH]uint16
	n, _ := windows.GetModuleFileName(windows.Handle(modcimfs.Handle()), &buf[0], uint32(len(buf)))
	if n > 0 {
		logrus.WithField("path", windows.UTF16ToString(buf[:n])).Info("loaded cimfs.dll")
	}
	return nil
})

// Supported checks if cimfs.dll is present on the system.
func Supported() bool {
	return load() == nil
}
