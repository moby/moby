package archive

import (
	"io"

	"github.com/moby/go-archive"
	"github.com/moby/go-archive/compression"
)

var (
	ErrNotDirectory      = archive.ErrNotDirectory      // Deprecated: use [archive.ErrNotDirectory] instead.
	ErrDirNotExists      = archive.ErrDirNotExists      // Deprecated: use [archive.ErrDirNotExists] instead.
	ErrCannotCopyDir     = archive.ErrCannotCopyDir     // Deprecated: use [archive.ErrCannotCopyDir] instead.
	ErrInvalidCopySource = archive.ErrInvalidCopySource // Deprecated: use [archive.ErrInvalidCopySource] instead.
)

// PreserveTrailingDotOrSeparator returns the given cleaned path.
//
// Deprecated: use [archive.PreserveTrailingDotOrSeparator] instead.
func PreserveTrailingDotOrSeparator(cleanedPath string, originalPath string) string {
	return archive.PreserveTrailingDotOrSeparator(cleanedPath, originalPath)
}

// SplitPathDirEntry splits the given path between its directory name and its
// basename.
//
// Deprecated: use [archive.SplitPathDirEntry] instead.
func SplitPathDirEntry(path string) (dir, base string) {
	return archive.SplitPathDirEntry(path)
}

// TarResource archives the resource described by the given CopyInfo to a Tar
// archive.
//
// Deprecated: use [archive.TarResource] instead.
func TarResource(sourceInfo archive.CopyInfo) (content io.ReadCloser, err error) {
	return archive.TarResource(sourceInfo)
}

// TarResourceRebase is like TarResource but renames the first path element of
// items in the resulting tar archive to match the given rebaseName if not "".
//
// Deprecated: use [archive.TarResourceRebase] instead.
func TarResourceRebase(sourcePath, rebaseName string) (content io.ReadCloser, _ error) {
	return archive.TarResourceRebase(sourcePath, rebaseName)
}

// TarResourceRebaseOpts does not preform the Tar, but instead just creates the rebase
// parameters to be sent to TarWithOptions.
//
// Deprecated: use [archive.TarResourceRebaseOpts] instead.
func TarResourceRebaseOpts(sourceBase string, rebaseName string) *TarOptions {
	filter := []string{sourceBase}
	return &TarOptions{
		Compression:      compression.None,
		IncludeFiles:     filter,
		IncludeSourceDir: true,
		RebaseNames: map[string]string{
			sourceBase: rebaseName,
		},
	}
}

// CopyInfo holds basic info about the source or destination path of a copy operation.
//
// Deprecated: use [archive.CopyInfo] instead.
type CopyInfo = archive.CopyInfo

// CopyInfoSourcePath stats the given path to create a CopyInfo struct.
// struct representing that resource for the source of an archive copy
// operation.
//
// Deprecated: use [archive.CopyInfoSourcePath] instead.
func CopyInfoSourcePath(path string, followLink bool) (archive.CopyInfo, error) {
	return archive.CopyInfoSourcePath(path, followLink)
}

// CopyInfoDestinationPath stats the given path to create a CopyInfo
// struct representing that resource for the destination of an archive copy
// operation.
//
// Deprecated: use [archive.CopyInfoDestinationPath] instead.
func CopyInfoDestinationPath(path string) (info archive.CopyInfo, err error) {
	return archive.CopyInfoDestinationPath(path)
}

// PrepareArchiveCopy prepares the given srcContent archive.
//
// Deprecated: use [archive.PrepareArchiveCopy] instead.
func PrepareArchiveCopy(srcContent io.Reader, srcInfo, dstInfo archive.CopyInfo) (dstDir string, content io.ReadCloser, err error) {
	return archive.PrepareArchiveCopy(srcContent, srcInfo, dstInfo)
}

// RebaseArchiveEntries rewrites the given srcContent archive replacing
// an occurrence of oldBase with newBase at the beginning of entry names.
//
// Deprecated: use [archive.RebaseArchiveEntries] instead.
func RebaseArchiveEntries(srcContent io.Reader, oldBase, newBase string) io.ReadCloser {
	return archive.RebaseArchiveEntries(srcContent, oldBase, newBase)
}

// CopyResource performs an archive copy from the given source path to the
// given destination path.
//
// Deprecated: use [archive.CopyResource] instead.
func CopyResource(srcPath, dstPath string, followLink bool) error {
	return archive.CopyResource(srcPath, dstPath, followLink)
}

// CopyTo handles extracting the given content whose
// entries should be sourced from srcInfo to dstPath.
//
// Deprecated: use [archive.CopyTo] instead.
func CopyTo(content io.Reader, srcInfo archive.CopyInfo, dstPath string) error {
	return archive.CopyTo(content, srcInfo, dstPath)
}

// ResolveHostSourcePath decides real path need to be copied.
//
// Deprecated: use [archive.ResolveHostSourcePath] instead.
func ResolveHostSourcePath(path string, followLink bool) (resolvedPath, rebaseName string, _ error) {
	return archive.ResolveHostSourcePath(path, followLink)
}

// GetRebaseName normalizes and compares path and resolvedPath.
//
// Deprecated: use [archive.GetRebaseName] instead.
func GetRebaseName(path, resolvedPath string) (string, string) {
	return archive.GetRebaseName(path, resolvedPath)
}
