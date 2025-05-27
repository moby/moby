// Package archive provides helper functions for dealing with archive files.
package archive

import (
	"archive/tar"
	"io"
	"os"

	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/go-archive"
	"github.com/moby/go-archive/compression"
	"github.com/moby/go-archive/tarheader"
)

// ImpliedDirectoryMode represents the mode (Unix permissions) applied to directories that are implied by files in a
// tar, but that do not have their own header entry.
//
// Deprecated: use [archive.ImpliedDirectoryMode] instead.
const ImpliedDirectoryMode = archive.ImpliedDirectoryMode

type (
	// Compression is the state represents if compressed or not.
	//
	// Deprecated: use [compression.Compression] instead.
	Compression = compression.Compression
	// WhiteoutFormat is the format of whiteouts unpacked
	//
	// Deprecated: use [archive.WhiteoutFormat] instead.
	WhiteoutFormat = archive.WhiteoutFormat

	// TarOptions wraps the tar options.
	//
	// Deprecated: use [archive.TarOptions] instead.
	TarOptions struct {
		IncludeFiles     []string
		ExcludePatterns  []string
		Compression      compression.Compression
		NoLchown         bool
		IDMap            idtools.IdentityMapping
		ChownOpts        *idtools.Identity
		IncludeSourceDir bool
		// WhiteoutFormat is the expected on disk format for whiteout files.
		// This format will be converted to the standard format on pack
		// and from the standard format on unpack.
		WhiteoutFormat archive.WhiteoutFormat
		// When unpacking, specifies whether overwriting a directory with a
		// non-directory is allowed and vice versa.
		NoOverwriteDirNonDir bool
		// For each include when creating an archive, the included name will be
		// replaced with the matching name from this map.
		RebaseNames map[string]string
		InUserNS    bool
		// Allow unpacking to succeed in spite of failures to set extended
		// attributes on the unpacked files due to the destination filesystem
		// not supporting them or a lack of permissions. Extended attributes
		// were probably in the archive for a reason, so set this option at
		// your own peril.
		BestEffortXattrs bool
	}
)

// Archiver implements the Archiver interface and allows the reuse of most utility functions of
// this package with a pluggable Untar function. Also, to facilitate the passing of specific id
// mappings for untar, an Archiver can be created with maps which will then be passed to Untar operations.
//
// Deprecated: use [archive.Archiver] instead.
type Archiver struct {
	Untar     func(io.Reader, string, *TarOptions) error
	IDMapping idtools.IdentityMapping
}

// NewDefaultArchiver returns a new Archiver without any IdentityMapping
//
// Deprecated: use [archive.NewDefaultArchiver] instead.
func NewDefaultArchiver() *Archiver {
	return &Archiver{Untar: Untar}
}

const (
	Uncompressed = compression.None  // Deprecated: use [compression.None] instead.
	Bzip2        = compression.Bzip2 // Deprecated: use [compression.Bzip2] instead.
	Gzip         = compression.Gzip  // Deprecated: use [compression.Gzip] instead.
	Xz           = compression.Xz    // Deprecated: use [compression.Xz] instead.
	Zstd         = compression.Zstd  // Deprecated: use [compression.Zstd] instead.
)

const (
	AUFSWhiteoutFormat    = archive.AUFSWhiteoutFormat    // Deprecated: use [archive.AUFSWhiteoutFormat] instead.
	OverlayWhiteoutFormat = archive.OverlayWhiteoutFormat // Deprecated: use [archive.OverlayWhiteoutFormat] instead.
)

// IsArchivePath checks if the (possibly compressed) file at the given path
// starts with a tar file header.
//
// Deprecated: use [archive.IsArchivePath] instead.
func IsArchivePath(path string) bool {
	return archive.IsArchivePath(path)
}

// DetectCompression detects the compression algorithm of the source.
//
// Deprecated: use [compression.Detect] instead.
func DetectCompression(source []byte) archive.Compression {
	return compression.Detect(source)
}

// DecompressStream decompresses the archive and returns a ReaderCloser with the decompressed archive.
//
// Deprecated: use [compression.DecompressStream] instead.
func DecompressStream(arch io.Reader) (io.ReadCloser, error) {
	return compression.DecompressStream(arch)
}

// CompressStream compresses the dest with specified compression algorithm.
//
// Deprecated: use [compression.CompressStream] instead.
func CompressStream(dest io.Writer, comp compression.Compression) (io.WriteCloser, error) {
	return compression.CompressStream(dest, comp)
}

// TarModifierFunc is a function that can be passed to ReplaceFileTarWrapper.
//
// Deprecated: use [archive.TarModifierFunc] instead.
type TarModifierFunc = archive.TarModifierFunc

// ReplaceFileTarWrapper converts inputTarStream to a new tar stream.
//
// Deprecated: use [archive.ReplaceFileTarWrapper] instead.
func ReplaceFileTarWrapper(inputTarStream io.ReadCloser, mods map[string]archive.TarModifierFunc) io.ReadCloser {
	return archive.ReplaceFileTarWrapper(inputTarStream, mods)
}

// FileInfoHeaderNoLookups creates a partially-populated tar.Header from fi.
//
// Deprecated: use [tarheader.FileInfoHeaderNoLookups] instead.
func FileInfoHeaderNoLookups(fi os.FileInfo, link string) (*tar.Header, error) {
	return tarheader.FileInfoHeaderNoLookups(fi, link)
}

// FileInfoHeader creates a populated Header from fi.
//
// Deprecated: use [archive.FileInfoHeader] instead.
func FileInfoHeader(name string, fi os.FileInfo, link string) (*tar.Header, error) {
	return archive.FileInfoHeader(name, fi, link)
}

// ReadSecurityXattrToTarHeader reads security.capability xattr from filesystem
// to a tar header
//
// Deprecated: use [archive.ReadSecurityXattrToTarHeader] instead.
func ReadSecurityXattrToTarHeader(path string, hdr *tar.Header) error {
	return archive.ReadSecurityXattrToTarHeader(path, hdr)
}

// Tar creates an archive from the directory at `path`, and returns it as a
// stream of bytes.
//
// Deprecated: use [archive.Tar] instead.
func Tar(path string, compression archive.Compression) (io.ReadCloser, error) {
	return archive.TarWithOptions(path, &archive.TarOptions{Compression: compression})
}

// TarWithOptions creates an archive with the given options.
//
// Deprecated: use [archive.TarWithOptions] instead.
func TarWithOptions(srcPath string, options *TarOptions) (io.ReadCloser, error) {
	return archive.TarWithOptions(srcPath, toArchiveOpt(options))
}

// Tarballer is a lower-level interface to TarWithOptions.
//
// Deprecated: use [archive.Tarballer] instead.
type Tarballer = archive.Tarballer

// NewTarballer constructs a new tarballer using TarWithOptions.
//
// Deprecated: use [archive.Tarballer] instead.
func NewTarballer(srcPath string, options *TarOptions) (*archive.Tarballer, error) {
	return archive.NewTarballer(srcPath, toArchiveOpt(options))
}

// Unpack unpacks the decompressedArchive to dest with options.
//
// Deprecated: use [archive.Unpack] instead.
func Unpack(decompressedArchive io.Reader, dest string, options *TarOptions) error {
	return archive.Unpack(decompressedArchive, dest, toArchiveOpt(options))
}

// Untar reads a stream of bytes from `archive`, parses it as a tar archive,
// and unpacks it into the directory at `dest`.
//
// Deprecated: use [archive.Untar] instead.
func Untar(tarArchive io.Reader, dest string, options *TarOptions) error {
	return archive.Untar(tarArchive, dest, toArchiveOpt(options))
}

// UntarUncompressed reads a stream of bytes from `tarArchive`, parses it as a tar archive,
// and unpacks it into the directory at `dest`.
// The archive must be an uncompressed stream.
//
// Deprecated: use [archive.UntarUncompressed] instead.
func UntarUncompressed(tarArchive io.Reader, dest string, options *TarOptions) error {
	return archive.UntarUncompressed(tarArchive, dest, toArchiveOpt(options))
}

// TarUntar is a convenience function which calls Tar and Untar, with the output of one piped into the other.
// If either Tar or Untar fails, TarUntar aborts and returns the error.
func (archiver *Archiver) TarUntar(src, dst string) error {
	return (&archive.Archiver{
		Untar: func(reader io.Reader, s string, options *archive.TarOptions) error {
			return archiver.Untar(reader, s, &TarOptions{
				IDMap: archiver.IDMapping,
			})
		},
		IDMapping: idtools.ToUserIdentityMapping(archiver.IDMapping),
	}).TarUntar(src, dst)
}

// UntarPath untar a file from path to a destination, src is the source tar file path.
func (archiver *Archiver) UntarPath(src, dst string) error {
	return (&archive.Archiver{
		Untar: func(reader io.Reader, s string, options *archive.TarOptions) error {
			return archiver.Untar(reader, s, &TarOptions{
				IDMap: archiver.IDMapping,
			})
		},
		IDMapping: idtools.ToUserIdentityMapping(archiver.IDMapping),
	}).UntarPath(src, dst)
}

// CopyWithTar creates a tar archive of filesystem path `src`, and
// unpacks it at filesystem path `dst`.
// The archive is streamed directly with fixed buffering and no
// intermediary disk IO.
func (archiver *Archiver) CopyWithTar(src, dst string) error {
	return (&archive.Archiver{
		Untar: func(reader io.Reader, s string, options *archive.TarOptions) error {
			return archiver.Untar(reader, s, nil)
		},
		IDMapping: idtools.ToUserIdentityMapping(archiver.IDMapping),
	}).CopyWithTar(src, dst)
}

// CopyFileWithTar emulates the behavior of the 'cp' command-line
// for a single file. It copies a regular file from path `src` to
// path `dst`, and preserves all its metadata.
func (archiver *Archiver) CopyFileWithTar(src, dst string) (err error) {
	return (&archive.Archiver{
		Untar: func(reader io.Reader, s string, options *archive.TarOptions) error {
			return archiver.Untar(reader, s, nil)
		},
		IDMapping: idtools.ToUserIdentityMapping(archiver.IDMapping),
	}).CopyFileWithTar(src, dst)
}

// IdentityMapping returns the IdentityMapping of the archiver.
func (archiver *Archiver) IdentityMapping() idtools.IdentityMapping {
	return archiver.IDMapping
}
