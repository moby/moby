//go:build windows

package winapi

import (
	"unsafe"

	"github.com/Microsoft/go-winio/pkg/guid"
	"golang.org/x/sys/windows"
)

type g = guid.GUID
type FsHandle uintptr
type StreamHandle uintptr

type CimFsFileMetadata struct {
	Attributes uint32
	FileSize   int64

	CreationTime   windows.Filetime
	LastWriteTime  windows.Filetime
	ChangeTime     windows.Filetime
	LastAccessTime windows.Filetime

	SecurityDescriptorBuffer unsafe.Pointer
	SecurityDescriptorSize   uint32

	ReparseDataBuffer unsafe.Pointer
	ReparseDataSize   uint32

	ExtendedAttributes unsafe.Pointer
	EACount            uint32
}

type CimFsImagePath struct {
	ImageDir  *uint16
	ImageName *uint16
}

//sys CimMountImage(imagePath string, fsName string, flags uint32, volumeID *g) (hr error) = cimfs.CimMountImage?
//sys CimDismountImage(volumeID *g) (hr error) = cimfs.CimDismountImage?

//sys CimCreateImage(imagePath string, oldFSName *uint16, newFSName *uint16, cimFSHandle *FsHandle) (hr error) = cimfs.CimCreateImage?
//sys CimCreateImage2(imagePath string, flags uint32, oldFSName *uint16, newFSName *uint16, cimFSHandle *FsHandle) (hr error) = cimfs.CimCreateImage2?
//sys CimCloseImage(cimFSHandle FsHandle) = cimfs.CimCloseImage?
//sys CimCommitImage(cimFSHandle FsHandle) (hr error) = cimfs.CimCommitImage?

//sys CimCreateFile(cimFSHandle FsHandle, path string, file *CimFsFileMetadata, cimStreamHandle *StreamHandle) (hr error) = cimfs.CimCreateFile?
//sys CimCloseStream(cimStreamHandle StreamHandle) (hr error) = cimfs.CimCloseStream?
//sys CimWriteStream(cimStreamHandle StreamHandle, buffer uintptr, bufferSize uint32) (hr error) = cimfs.CimWriteStream?
//sys CimDeletePath(cimFSHandle FsHandle, path string) (hr error) = cimfs.CimDeletePath?
//sys CimCreateHardLink(cimFSHandle FsHandle, newPath string, oldPath string) (hr error) = cimfs.CimCreateHardLink?
//sys CimCreateAlternateStream(cimFSHandle FsHandle, path string, size uint64, cimStreamHandle *StreamHandle) (hr error) = cimfs.CimCreateAlternateStream?
//sys CimAddFsToMergedImage(cimFSHandle FsHandle, path string) (hr error) = cimfs.CimAddFsToMergedImage?
//sys CimAddFsToMergedImage2(cimFSHandle FsHandle, path string, flags uint32) (hr error) = cimfs.CimAddFsToMergedImage2?
//sys CimMergeMountImage(numCimPaths uint32, backingImagePaths *CimFsImagePath, flags uint32, volumeID *g) (hr error) = cimfs.CimMergeMountImage?
//sys CimTombstoneFile(cimFSHandle FsHandle, path string) (hr error) = cimfs.CimTombstoneFile?
//sys CimCreateMergeLink(cimFSHandle FsHandle, newPath string, oldPath string) (hr error) = cimfs.CimCreateMergeLink?
//sys CimSealImage(blockCimPath string, hashSize *uint64, fixedHeaderSize *uint64, hash *byte) (hr error) = cimfs.CimSealImage?
//sys CimGetVerificationInformation(blockCimPath string, isSealed *uint32, hashSize *uint64, signatureSize *uint64, fixedHeaderSize *uint64, hash *byte, signature *byte) (hr error) = cimfs.CimGetVerificationInformation?
//sys CimMountVerifiedImage(imagePath string, fsName string, flags uint32, volumeID *g, hashSize uint16, hash *byte) (hr error) = cimfs.CimMountVerifiedImage?
