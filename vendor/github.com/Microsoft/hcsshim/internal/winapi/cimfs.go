//go:build windows

package winapi

import (
	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/sirupsen/logrus"

	"github.com/Microsoft/hcsshim/internal/winapi/cimfs"
	"github.com/Microsoft/hcsshim/internal/winapi/cimwriter"
	"github.com/Microsoft/hcsshim/internal/winapi/types"
)

// LogCimDLLSupport logs which DLL is being used for CIM write operations.
func LogCimDLLSupport() {
	if cimwriter.Supported() {
		logrus.Info("using cimwriter.dll for CIM write operations")
	} else if cimfs.Supported() {
		logrus.Info("using cimfs.dll for CIM write operations")
	} else {
		logrus.Warn("no CIM DLL available for write operations")
	}
}

// pickSupported makes sure we use appropriate syscalls depending on which DLLs are present.
func pickSupported[F any](cimWriterFunc, cimfsFunc F) F {
	if cimwriter.Supported() {
		return cimWriterFunc
	}
	return cimfsFunc
}

func CimMountImage(imagePath string, fsName string, flags uint32, volumeID *guid.GUID) error {
	return cimfs.CimMountImage(imagePath, fsName, flags, volumeID)
}

func CimDismountImage(volumeID *guid.GUID) error {
	return cimfs.CimDismountImage(volumeID)
}

func CimCreateImage(imagePath string, oldFSName *uint16, newFSName *uint16, cimFSHandle *types.FsHandle) error {
	return pickSupported(
		cimwriter.CimCreateImage,
		cimfs.CimCreateImage,
	)(imagePath, oldFSName, newFSName, cimFSHandle)
}

func CimCreateImage2(imagePath string, flags uint32, oldFSName *uint16, newFSName *uint16, cimFSHandle *types.FsHandle) error {
	return pickSupported(
		cimwriter.CimCreateImage2,
		cimfs.CimCreateImage2,
	)(imagePath, flags, oldFSName, newFSName, cimFSHandle)
}

func CimCloseImage(cimFSHandle types.FsHandle) error {
	return pickSupported(
		cimwriter.CimCloseImage,
		cimfs.CimCloseImage,
	)(cimFSHandle)
}

func CimCommitImage(cimFSHandle types.FsHandle) error {
	return pickSupported(
		cimwriter.CimCommitImage,
		cimfs.CimCommitImage,
	)(cimFSHandle)
}

func CimCreateFile(cimFSHandle types.FsHandle, path string, file *types.CimFsFileMetadata, cimStreamHandle *types.StreamHandle) error {
	return pickSupported(
		cimwriter.CimCreateFile,
		cimfs.CimCreateFile,
	)(cimFSHandle, path, file, cimStreamHandle)
}

func CimCloseStream(cimStreamHandle types.StreamHandle) error {
	return pickSupported(
		cimwriter.CimCloseStream,
		cimfs.CimCloseStream,
	)(cimStreamHandle)
}

func CimWriteStream(cimStreamHandle types.StreamHandle, buffer uintptr, bufferSize uint32) error {
	return pickSupported(
		cimwriter.CimWriteStream,
		cimfs.CimWriteStream,
	)(cimStreamHandle, buffer, bufferSize)
}

func CimDeletePath(cimFSHandle types.FsHandle, path string) error {
	return pickSupported(
		cimwriter.CimDeletePath,
		cimfs.CimDeletePath,
	)(cimFSHandle, path)
}

func CimCreateHardLink(cimFSHandle types.FsHandle, newPath string, oldPath string) error {
	return pickSupported(
		cimwriter.CimCreateHardLink,
		cimfs.CimCreateHardLink,
	)(cimFSHandle, newPath, oldPath)
}

func CimCreateAlternateStream(cimFSHandle types.FsHandle, path string, size uint64, cimStreamHandle *types.StreamHandle) error {
	return pickSupported(
		cimwriter.CimCreateAlternateStream,
		cimfs.CimCreateAlternateStream,
	)(cimFSHandle, path, size, cimStreamHandle)
}

func CimAddFsToMergedImage(cimFSHandle types.FsHandle, path string) error {
	return pickSupported(
		cimwriter.CimAddFsToMergedImage,
		cimfs.CimAddFsToMergedImage,
	)(cimFSHandle, path)
}

func CimAddFsToMergedImage2(cimFSHandle types.FsHandle, path string, flags uint32) error {
	return pickSupported(
		cimwriter.CimAddFsToMergedImage2,
		cimfs.CimAddFsToMergedImage2,
	)(cimFSHandle, path, flags)
}

func CimMergeMountImage(numCimPaths uint32, backingImagePaths *types.CimFsImagePath, flags uint32, volumeID *guid.GUID) error {
	return cimfs.CimMergeMountImage(numCimPaths, backingImagePaths, flags, volumeID)
}

func CimTombstoneFile(cimFSHandle types.FsHandle, path string) error {
	return pickSupported(
		cimwriter.CimTombstoneFile,
		cimfs.CimTombstoneFile,
	)(cimFSHandle, path)
}

func CimCreateMergeLink(cimFSHandle types.FsHandle, newPath string, oldPath string) (hr error) {
	return pickSupported(
		cimwriter.CimCreateMergeLink,
		cimfs.CimCreateMergeLink,
	)(cimFSHandle, newPath, oldPath)
}

func CimSealImage(blockCimPath string, hashSize *uint64, fixedHeaderSize *uint64, hash *byte) (hr error) {
	return pickSupported(
		cimwriter.CimSealImage,
		cimfs.CimSealImage,
	)(blockCimPath, hashSize, fixedHeaderSize, hash)
}

func CimGetVerificationInformation(blockCimPath string, isSealed *uint32, hashSize *uint64, signatureSize *uint64, fixedHeaderSize *uint64, hash *byte, signature *byte) (hr error) {
	return pickSupported(
		cimwriter.CimGetVerificationInformation,
		cimfs.CimGetVerificationInformation,
	)(blockCimPath, isSealed, hashSize, signatureSize, fixedHeaderSize, hash, signature)
}

func CimMountVerifiedImage(imagePath string, fsName string, flags uint32, volumeID *guid.GUID, hashSize uint16, hash *byte) error {
	return cimfs.CimMountVerifiedImage(imagePath, fsName, flags, volumeID, hashSize, hash)
}

func CimMergeMountVerifiedImage(numCimPaths uint32, backingImagePaths *types.CimFsImagePath, flags uint32, volumeID *guid.GUID, hashSize uint16, hash *byte) error {
	return cimfs.CimMergeMountVerifiedImage(numCimPaths, backingImagePaths, flags, volumeID, hashSize, hash)
}
