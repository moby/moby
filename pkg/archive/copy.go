package archive

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/docker/vendor/src/code.google.com/p/go/src/pkg/archive/tar"
)

// Errors used or returned by this file.
var (
	ErrNotDirectory      = errors.New("not a directory")
	ErrDirNotExists      = errors.New("no such directory")
	ErrCannotCopyDir     = errors.New("cannot copy directory")
	ErrInvalidCopySource = errors.New("invalid copy source content")
)

// PreserveTrailingDotOrSeparator returns the given cleaned path (after
// processing using any utility functions from the path or filepath stdlib
// packages) and appends a trailing `/.` or `/` if its corresponding  original
// path (from before being processed by utility functions from the path or
// filepath stdlib packages) ends with a trailing `/.` or `/`. If the cleaned
// path already ends in a `.` path segment, then another is not added. If the
// clean path already ends in a path separator, then another is not added.
func PreserveTrailingDotOrSeparator(cleanedPath, originalPath string) string {
	if !SpecifiesCurrentDir(cleanedPath) && SpecifiesCurrentDir(originalPath) {
		cleanedPath = fmt.Sprintf("%s%c.", cleanedPath, filepath.Separator)
	}

	if !HasTrailingPathSeparator(cleanedPath) && HasTrailingPathSeparator(originalPath) {
		cleanedPath = fmt.Sprintf("%s%c", cleanedPath, filepath.Separator)
	}

	return cleanedPath
}

// AssertsDirectory returns whether the given path is
// asserted to be a directory, i.e., the path ends with
// a trailing '/' or `/.`, assuming a path separator of `/`.
func AssertsDirectory(path string) bool {
	return HasTrailingPathSeparator(path) || SpecifiesCurrentDir(path)
}

// HasTrailingPathSeparator returns whether the given
// path ends with the system's path separator character.
func HasTrailingPathSeparator(path string) bool {
	return strings.HasSuffix(path, string(filepath.Separator))
}

// SpecifiesCurrentDir returns whether the given path specifies
// a "current directory", i.e., the last path segment is `.`.
func SpecifiesCurrentDir(path string) bool {
	trimmedPath := strings.TrimRight(path, string(filepath.Separator))
	currentDirIndicator := fmt.Sprintf("%c.", filepath.Separator)

	return strings.HasSuffix(trimmedPath, currentDirIndicator)
}

// CopyFrom copies the resource at the given sourcePath into a Tar archive.
// If sourcePath ends with a path separator, then only the contents of the
// directory will be included in the archive and not the directory itself, but
// it is an error if the resource at sourcePath is not a directory. If the
// sourcePath does not exist then os.IsNotExist(err) == true.
func CopyFrom(sourcePath string) (content Archive, err error) {
	return CopyFromReplaceBase(sourcePath, "")
}

// CopyFromReplaceBase copies the resource at the given sourcePath into a Tar
// archive. If sourcePath ends with a path separator, then only the contents of
// the directory will be included in the archive and not the directory itself,
// but it is an error if the resource at sourcePath is not a directory. If the
// sourcePath does not exist then os.IsNotExist(err) == true.
//
// If baseName is not empty, then the resulting archive will be created using
// it as the beginning of items relative paths. Calling CopyFromReplaceBase
// with an empty baseName is identical to calling CopyFrom with sourcePath.
func CopyFromReplaceBase(sourcePath, baseName string) (content Archive, err error) {
	var filter []string

	if _, err = os.Stat(sourcePath); err != nil {
		// Catches the case where the source does not exist
		// or is not a directory if asserted to be a directory.
		return
	}

	var replace [2]string
	if baseName != "" {
		// The caller has asked that we replace the baseName of the source
		// with the given baseName when adding files to the archive.
		_, originalBase := filepath.Split(sourcePath)
		replace = [2]string{originalBase, baseName}
	}

	// If sourcePath specifies a "current directory" then we  don't want to
	// include the directory itself in the archive. There's nothing to filter
	// because we want everything in that directory.
	if !SpecifiesCurrentDir(sourcePath) {
		// Need to filter to just the basename.
		var basename string
		// Clean and split the sourcePath between the parent directory and
		// the basename of the file or directory to be copied.
		sourcePath, basename = filepath.Split(filepath.Clean(sourcePath))
		filter = []string{basename}
	}

	log.Debugf("copying from %q: filter: %v, replace: %v", sourcePath, filter, replace)

	return TarWithOptions(sourcePath, &TarOptions{
		Compression:      Uncompressed,
		IncludeFiles:     filter,
		Replace:          replace,
		IncludeSourceDir: true,
	})
}

// CopyTo copies the content from the given archive to dstPath. It is an error
// if dstPath's parent directory does not exist. The resulting copy action is
// dependent on the content of the archive, specifically, the first entry
// header in the content archive is inspected to determine if it is a
// directory:
//
// - If the first item is *not* a directory then the content archive is assumed
//   to be a single file. The second entry will not be read. It is an error if
//   the dirname of the file is not empty.
//
// - If the first item in the archive is a directory then the content archive
//   is assumed to be a copy of a directory *and* its contents.
//
// - If the first item in the archive is a directory and its name is `./` then
//   the content archive is assumed to be a copy of only a directory's content.
//
// The dstPath argument is then checked for validity of the copy operation.
//
// - dstPath's parent directories MUST exist.
//
// - If dstPath exists as a directory, then the content of the archive is
//   simply extracted into this directory.
//
// - If dstPath exists as a file it is an error if dstPath is asserted to be a
//   directory or if the content of the archive is not determined to be a
//   single file, otherwise, the dstPath file is overwritten with the content
//   of the source file.
//
// - If dstPath does not exist and is asserted to be a directory, it is an
//   error if the source archive is determined to be a single file, otherwise,
//   the dstPath directory is created and the *contents* of the source
//   directory are copied into it.
//
// - If dstPath does not exist and *is not* asserted to be a directory, it is
//   an error if the source archive *is not* determined to be a single file,
//   otherwise the dstPath file is created with the source file's content.
//
func (archiver *Archiver) CopyTo(content ArchiveReader, dstPath string) (err error) {
	options := &TarOptions{NoLchown: true}

	// First, ensure that dstPath's parent directory exists.
	if err = ensureParentDirExists(dstPath); err != nil {
		return
	}

	var decompressedContent io.ReadCloser
	if decompressedContent, err = DecompressStream(content); err != nil {
		log.Errorf("unable to decompress copied content archive: %s", err)
		return
	}
	defer decompressedContent.Close()

	// Next, wrap and inspect the content archive's first entry.
	wrapper := &copiedContentArchiveWrapper{underlyingArchive: decompressedContent}
	wrapper.detectCopiedType()

	switch wrapper.copiedType {
	case copiedTypeFile:
		return copyFileTo(archiver, wrapper, dstPath, options)
	case copiedTypeDir:
		return copyDirTo(archiver, wrapper, dstPath, options)
	default:
		return ErrInvalidCopySource
	}
}

// copyFileTo handles copying the first file from the given archive to the
// destination path. content should have been confirmed to be a single file by
// the outer call to CopyTo(). It extracts only the first entry from the given
// content archive to the specified dstPath preserving all its metadata.
//
// The entry must not have a TypeFlag of TypeDir. If dstPath ends with a
// trailing path separator, then it is interpreted as a directory and created
// if necessary. If dstPath exists and is a directory, the file will be
// extracted to that directory using the same basename, otherwise the file will
// be copied using the name in dstPath.
func copyFileTo(archiver *Archiver, content *copiedContentArchiveWrapper, dstPath string, options *TarOptions) error {
	log.Debugf("copying file to %q", dstPath)

	var dstDir, dstBase string

	// Determine if dstPath exists and is a directory.
	// The error is not nil if the destination does not exist, if it exists as
	// a file *but* dstPath is asserted to be a directory, or any other stat
	// error.
	dstStat, err := os.Stat(dstPath)

	switch {
	case err == nil && dstStat.IsDir():
		// Exists and is a directory. Use basename from the archive entry.
		dstDir = dstPath
	case err == nil:
		// Exists and is a file. Use the destination basename.
		dstDir, dstBase = filepath.Split(dstPath)
	case os.IsNotExist(err) && AssertsDirectory(dstPath):
		// Do Not create the directory, just return the appropirate error.
		return ErrDirNotExists
	case os.IsNotExist(err):
		// The parent directory is confirmed to exist by the outer call to
		// CopyTo(). The destination file will be created when the archive is
		// extracted to the parent directory. Use the destination basename.
		dstDir, dstBase = filepath.Split(dstPath)
	default:
		return err
	}

	// We may need to swap the Name of the entry to match the dstPath so an
	// altered tar archive will need to be constructed. We also want to ensure
	// that we extract exactly one entry.
	r, w := io.Pipe()
	errC := promise.Go(func() error {
		defer w.Close()

		tarReader := tar.NewReader(content)
		hdr, err := tarReader.Next()
		if err != nil {
			return err
		}

		if hdr.Typeflag == tar.TypeDir || filepath.Dir(hdr.Name) != "." {
			return ErrCannotCopyDir
		}

		// Alter the basename of the file if specified.
		if dstBase != "" {
			hdr.Name = dstBase
		}

		tarWriter := tar.NewWriter(w)
		defer tarWriter.Close()

		if err = tarWriter.WriteHeader(hdr); err != nil {
			return err
		}

		// Fully copy only the first entry to the tar writer.
		_, err = io.Copy(tarWriter, tarReader)

		return err
	})

	defer func() {
		if er := <-errC; err != nil {
			err = er
		}
	}()

	return archiver.Untar(r, dstDir, options)
}

// copyDirTo handles copying the directory from the given archive to  the
// destination path. content should have been confirmed to be a directory by
// the outer call to CopyTo(). When unpacking, contents are filtered to only
// the directory which the given wrapper detected from the content archive. The
// directory and its contents are copied to the specified dstPath preserving
// all metadata.
//
// If dstPath does not exist, when unpacking, the dirname is replaced by the
// basename of dstPath, and the archive is extracted into dstPath's parent
// directory. It is an error if dstPath exists and is not a directory.
func copyDirTo(archiver *Archiver, content *copiedContentArchiveWrapper, dstPath string, options *TarOptions) error {
	log.Debugf("copying directory to %q", dstPath)

	// Determine if dstPath exists and is a directory.
	// The error is not nil if the destination does not exist, if it exists as
	// a file *but* dstPath is asserted to be a directory, or any other stat
	// error.
	dstStat, err := os.Stat(dstPath)

	var dstDir string

	switch {
	case err == nil && dstStat.IsDir():
		// Exists and is a directory.
		dstDir = dstPath
	case err == nil:
		// Exists and is a file.
		return ErrNotDirectory
	case os.IsNotExist(err):
		// The parent directory is confirmed to exist by the outer call to
		// CopyTo(). The destination directory will be created when the archive
		// is extracted to the parent directory.
		dstDir = filepath.Dir(filepath.Clean(dstPath))
		options.Replace = [2]string{content.dirname, filepath.Base(filepath.Clean(dstPath))}
	default:
		return err
	}

	options.IncludeFiles = []string{content.dirname + "/"}

	log.Debugf("copying dir to %q: filter: %v, replace: %v", dstDir, options.IncludeFiles, options.Replace)

	return archiver.Untar(content, dstDir, options)
}

// CopyTo calls CopyTo on this package's default archiver.
func CopyTo(content ArchiveReader, dstPath string) (err error) {
	return defaultArchiver.CopyTo(content, dstPath)
}

func ensureParentDirExists(path string) error {
	parentDir := filepath.Dir(filepath.Clean(path))

	dirInfo, err := os.Stat(parentDir)
	if err != nil {
		// Either os.IsNotExist() or some other error.
		return err
	}

	if !dirInfo.IsDir() {
		return ErrNotDirectory
	}

	return nil
}

type copiedType int

const (
	// Zero value is invalid.
	copiedTypeInvalid copiedType = iota
	copiedTypeFile
	copiedTypeDir
)

// copiedContentArchiveWrapper is a convenient type which wraps an archive
// which is intended to be used by CopyTo above. The header of the first entry
// of the archive is inspected to determine if the content being copied is
// a single file, a directory, or only a directory's contents.
type copiedContentArchiveWrapper struct {
	underlyingArchive ArchiveReader
	buf               bytes.Buffer
	checked           bool
	copiedType        copiedType
	dirname           string
}

func (w *copiedContentArchiveWrapper) detectCopiedType() {
	if w.checked {
		// Be sure not to inspect the archive more than once!
		return
	}

	w.checked = true

	// Make a new Tar reader, but save what's read to the internal buffer.
	tarReader := tar.NewReader(io.TeeReader(w.underlyingArchive, &w.buf))

	hdr, err := tarReader.Next()
	if err != nil {
		log.Error("unable to read copied content archive")
		// Invalid copied content.
		return
	}

	// Get the dirname and basename of the entry.
	w.dirname = filepath.Dir(hdr.Name)

	if hdr.Typeflag == tar.TypeDir {
		w.copiedType = copiedTypeDir
	} else if w.dirname == "." {
		// Copied content is just this single file, no directory.
		w.copiedType = copiedTypeFile
	} else {
		// Invalid copied content.
		log.Errorf("copied content file not from `.` directory: %q", hdr.Name)
	}
}

func (w *copiedContentArchiveWrapper) Read(p []byte) (n int, err error) {
	// The buffer can only return EOF or nil.
	bufN, _ := w.buf.Read(p)

	if bufN < len(p) {
		n, err = w.underlyingArchive.Read(p[bufN:])
	}

	n += bufN

	return
}
