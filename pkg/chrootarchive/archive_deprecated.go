package chrootarchive

import (
	"io"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/go-archive/chrootarchive"
)

// NewArchiver returns a new Archiver which uses chrootarchive.Untar
//
// Deprecated: use [chrootarchive.NewArchiver] instead.
func NewArchiver(idMapping idtools.IdentityMapping) *archive.Archiver {
	return &archive.Archiver{
		Untar:     Untar,
		IDMapping: idMapping,
	}
}

// Untar reads a stream of bytes from `archive`, parses it as a tar archive,
// and unpacks it into the directory at `dest`.
//
// Deprecated: use [chrootarchive.Untar] instead.
func Untar(tarArchive io.Reader, dest string, options *archive.TarOptions) error {
	return chrootarchive.Untar(tarArchive, dest, archive.ToArchiveOpt(options))
}

// UntarWithRoot is the same as [Untar], but allows you to pass in a root directory.
//
// Deprecated: use [chrootarchive.UntarWithRoot] instead.
func UntarWithRoot(tarArchive io.Reader, dest string, options *archive.TarOptions, root string) error {
	return chrootarchive.UntarWithRoot(tarArchive, dest, archive.ToArchiveOpt(options), root)
}

// UntarUncompressed reads a stream of bytes from tarArchive, parses it as a tar archive,
// and unpacks it into the directory at dest.
//
// Deprecated: use [chrootarchive.UntarUncompressed] instead.
func UntarUncompressed(tarArchive io.Reader, dest string, options *archive.TarOptions) error {
	return chrootarchive.UntarUncompressed(tarArchive, dest, archive.ToArchiveOpt(options))
}

// Tar tars the requested path while chrooted to the specified root.
//
// Deprecated: use [chrootarchive.Tar] instead.
func Tar(srcPath string, options *archive.TarOptions, root string) (io.ReadCloser, error) {
	return chrootarchive.Tar(srcPath, archive.ToArchiveOpt(options), root)
}
