//go:build windows

package cimwriter

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"

	"github.com/Microsoft/hcsshim/internal/winapi/types"
)

type FsHandle = types.FsHandle
type StreamHandle = types.StreamHandle
type FileMetadata = types.CimFsFileMetadata
type ImagePath = types.CimFsImagePath

//sys CimCreateImage(imagePath string, oldFSName *uint16, newFSName *uint16, cimFSHandle *FsHandle) (hr error) = cimwriter.CimCreateImage?
//sys CimCreateImage2(imagePath string, flags uint32, oldFSName *uint16, newFSName *uint16, cimFSHandle *FsHandle) (hr error) = cimwriter.CimCreateImage2?
//sys CimCloseImage(cimFSHandle FsHandle) = cimwriter.CimCloseImage?
//sys CimCommitImage(cimFSHandle FsHandle) (hr error) = cimwriter.CimCommitImage?

//sys CimCreateFile(cimFSHandle FsHandle, path string, file *FileMetadata, cimStreamHandle *StreamHandle) (hr error) = cimwriter.CimCreateFile?
//sys CimCloseStream(cimStreamHandle StreamHandle) (hr error) = cimwriter.CimCloseStream?
//sys CimWriteStream(cimStreamHandle StreamHandle, buffer uintptr, bufferSize uint32) (hr error) = cimwriter.CimWriteStream?
//sys CimDeletePath(cimFSHandle FsHandle, path string) (hr error) = cimwriter.CimDeletePath?
//sys CimCreateHardLink(cimFSHandle FsHandle, newPath string, oldPath string) (hr error) = cimwriter.CimCreateHardLink?
//sys CimCreateAlternateStream(cimFSHandle FsHandle, path string, size uint64, cimStreamHandle *StreamHandle) (hr error) = cimwriter.CimCreateAlternateStream?
//sys CimAddFsToMergedImage(cimFSHandle FsHandle, path string) (hr error) = cimwriter.CimAddFsToMergedImage?
//sys CimAddFsToMergedImage2(cimFSHandle FsHandle, path string, flags uint32) (hr error) = cimwriter.CimAddFsToMergedImage2?

//sys CimTombstoneFile(cimFSHandle FsHandle, path string) (hr error) = cimwriter.CimTombstoneFile?
//sys CimCreateMergeLink(cimFSHandle FsHandle, newPath string, oldPath string) (hr error) = cimwriter.CimCreateMergeLink?
//sys CimSealImage(blockCimPath string, hashSize *uint64, fixedHeaderSize *uint64, hash *byte) (hr error) = cimwriter.CimSealImage?

//sys CimGetVerificationInformation(blockCimPath string, isSealed *uint32, hashSize *uint64, signatureSize *uint64, fixedHeaderSize *uint64, hash *byte, signature *byte) (hr error) = cimwriter.CimGetVerificationInformation?

var load = sync.OnceValue(func() error {
	// Pre-load the DLL with a restricted search path (System32 + application directory only)
	// to prevent loading from untrusted locations (e.g., CWD or arbitrary PATH entries).
	// The subsequent modcimwriter.Load() will reuse the already-loaded module.
	h, err := windows.LoadLibraryEx("cimwriter.dll", 0, windows.LOAD_LIBRARY_SEARCH_SYSTEM32|windows.LOAD_LIBRARY_SEARCH_APPLICATION_DIR)
	if err != nil {
		return err
	}
	if err := modcimwriter.Load(); err != nil {
		if freeErr := windows.FreeLibrary(h); freeErr != nil {
			logrus.WithError(freeErr).Warn("failed to free cimwriter.dll after load failure")
		}
		return err
	}
	var buf [windows.MAX_PATH]uint16
	n, _ := windows.GetModuleFileName(windows.Handle(modcimwriter.Handle()), &buf[0], uint32(len(buf)))
	if n > 0 {
		path := windows.UTF16ToString(buf[:n])
		fields := logrus.Fields{"path": path}
		if ver, err := getFileVersion(path); err == nil {
			fields["version"] = ver
		}
		logrus.WithFields(fields).Info("loaded cimwriter.dll")
	}
	return nil
})

// Supported checks if cimwriter.dll is present on the system.
func Supported() bool {
	return load() == nil
}

// getFileVersion returns the file version string (e.g. "10.0.26100.1") for the given path.
func getFileVersion(path string) (string, error) {
	size, err := windows.GetFileVersionInfoSize(path, nil)
	if err != nil {
		return "", err
	}
	data := make([]byte, size)
	if err := windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&data[0])); err != nil {
		return "", err
	}
	var info *windows.VS_FIXEDFILEINFO
	var infoLen uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&data[0]), `\`, unsafe.Pointer(&info), &infoLen); err != nil {
		return "", err
	}
	return formatFileVersion(info), nil
}

// formatFileVersion formats the file version from a VS_FIXEDFILEINFO as "major.minor.build.revision".
// See https://learn.microsoft.com/en-us/windows/win32/api/verrsrc/ns-verrsrc-vs_fixedfileinfo
func formatFileVersion(info *windows.VS_FIXEDFILEINFO) string {
	major := info.FileVersionMS >> 16
	minor := info.FileVersionMS & 0xffff
	build := info.FileVersionLS >> 16
	revision := info.FileVersionLS & 0xffff
	return fmt.Sprintf("%d.%d.%d.%d", major, minor, build, revision)
}
