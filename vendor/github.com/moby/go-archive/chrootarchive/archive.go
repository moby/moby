package chrootarchive

import (
	"io"

	"github.com/moby/sys/user"

	"github.com/moby/go-archive"
)

// NewArchiver returns a new Archiver which uses chrootarchive.Untar
func NewArchiver(idMapping user.IdentityMapping) *archive.Archiver {
	return &archive.Archiver{
		Untar:     Untar,
		IDMapping: idMapping,
	}
}

// Untar reads a stream of bytes from `archive`, parses it as a tar archive,
// and unpacks it into the directory at `dest`.
// The archive may be compressed with one of the following algorithms:
// identity (uncompressed), gzip, bzip2, xz.
func Untar(tarArchive io.Reader, dest string, options *archive.TarOptions) error {
	return untarHandler(tarArchive, dest, options, true, dest)
}

// UntarWithRoot is the same as `Untar`, but allows you to pass in a root directory
// The root directory is the directory that will be chrooted to.
// `dest` must be a path within `root`, if it is not an error will be returned.
//
// `root` should set to a directory which is not controlled by any potentially
// malicious process.
//
// This should be used to prevent a potential attacker from manipulating `dest`
// such that it would provide access to files outside of `dest` through things
// like symlinks. Normally `ResolveSymlinksInScope` would handle this, however
// sanitizing symlinks in this manner is inherently racey:
// ref: CVE-2018-15664
func UntarWithRoot(tarArchive io.Reader, dest string, options *archive.TarOptions, root string) error {
	return untarHandler(tarArchive, dest, options, true, root)
}

// UntarUncompressed reads a stream of bytes from `archive`, parses it as a tar archive,
// and unpacks it into the directory at `dest`.
// The archive must be an uncompressed stream.
func UntarUncompressed(tarArchive io.Reader, dest string, options *archive.TarOptions) error {
	return untarHandler(tarArchive, dest, options, false, dest)
}

// Tar tars the requested path while chrooted to the specified root.
func Tar(srcPath string, options *archive.TarOptions, root string) (io.ReadCloser, error) {
	if options == nil {
		options = &archive.TarOptions{}
	}
	return invokePack(srcPath, options, root)
}
