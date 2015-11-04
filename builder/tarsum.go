package builder

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/symlink"
	"github.com/docker/docker/pkg/tarsum"
)

type tarSumContext struct {
	root string
	sums tarsum.FileInfoSums
}

func (c *tarSumContext) Close() error {
	return os.RemoveAll(c.root)
}

func convertPathError(err error, cleanpath string) error {
	if err, ok := err.(*os.PathError); ok {
		err.Path = cleanpath
		return err
	}
	return err
}

func (c *tarSumContext) Open(path string) (io.ReadCloser, error) {
	cleanpath, fullpath, err := c.normalize(path)
	if err != nil {
		return nil, err
	}
	r, err := os.Open(fullpath)
	if err != nil {
		return nil, convertPathError(err, cleanpath)
	}
	return r, nil
}

func (c *tarSumContext) Stat(path string) (string, FileInfo, error) {
	cleanpath, fullpath, err := c.normalize(path)
	if err != nil {
		return "", nil, err
	}

	st, err := os.Lstat(fullpath)
	if err != nil {
		return "", nil, convertPathError(err, cleanpath)
	}

	rel, err := filepath.Rel(c.root, fullpath)
	if err != nil {
		return "", nil, convertPathError(err, cleanpath)
	}

	// We set sum to path by default for the case where GetFile returns nil.
	// The usual case is if relative path is empty.
	sum := path
	// Use the checksum of the followed path(not the possible symlink) because
	// this is the file that is actually copied.
	if tsInfo := c.sums.GetFile(rel); tsInfo != nil {
		sum = tsInfo.Sum()
	}
	fi := &HashedFileInfo{PathFileInfo{st, fullpath, filepath.Base(cleanpath)}, sum}
	return rel, fi, nil
}

// MakeTarSumContext returns a build Context from a tar stream.
//
// It extracts the tar stream to a temporary folder that is deleted as soon as
// the Context is closed.
// As the extraction happens, a tarsum is calculated for every file, and the set of
// all those sums then becomes the source of truth for all operations on this Context.
//
// Closing tarStream has to be done by the caller.
func MakeTarSumContext(tarStream io.Reader) (ModifiableContext, error) {
	root, err := ioutils.TempDir("", "docker-builder")
	if err != nil {
		return nil, err
	}

	tsc := &tarSumContext{root: root}

	// Make sure we clean-up upon error.  In the happy case the caller
	// is expected to manage the clean-up
	defer func() {
		if err != nil {
			tsc.Close()
		}
	}()

	decompressedStream, err := archive.DecompressStream(tarStream)
	if err != nil {
		return nil, err
	}

	sum, err := tarsum.NewTarSum(decompressedStream, true, tarsum.Version1)
	if err != nil {
		return nil, err
	}

	if err := chrootarchive.Untar(sum, root, nil); err != nil {
		return nil, err
	}

	tsc.sums = sum.GetSums()

	return tsc, nil
}

func (c *tarSumContext) normalize(path string) (cleanpath, fullpath string, err error) {
	cleanpath = filepath.Clean(string(os.PathSeparator) + path)[1:]
	fullpath, err = symlink.FollowSymlinkInScope(filepath.Join(c.root, path), c.root)
	if err != nil {
		return "", "", fmt.Errorf("Forbidden path outside the build context: %s (%s)", path, fullpath)
	}
	_, err = os.Lstat(fullpath)
	if err != nil {
		return "", "", convertPathError(err, path)
	}
	return
}

func (c *tarSumContext) Walk(root string, walkFn WalkFunc) error {
	root = filepath.Join(c.root, filepath.Join(string(filepath.Separator), root))
	return filepath.Walk(root, func(fullpath string, info os.FileInfo, err error) error {
		rel, err := filepath.Rel(c.root, fullpath)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		sum := rel
		if tsInfo := c.sums.GetFile(rel); tsInfo != nil {
			sum = tsInfo.Sum()
		}
		fi := &HashedFileInfo{PathFileInfo{FileInfo: info, FilePath: fullpath}, sum}
		if err := walkFn(rel, fi, nil); err != nil {
			return err
		}
		return nil
	})
}

func (c *tarSumContext) Remove(path string) error {
	_, fullpath, err := c.normalize(path)
	if err != nil {
		return err
	}
	return os.RemoveAll(fullpath)
}
