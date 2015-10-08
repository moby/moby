package builder

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

func (c *tarSumContext) Stat(path string) (fi FileInfo, err error) {
	cleanpath, fullpath, err := c.normalize(path)
	if err != nil {
		return nil, err
	}

	st, err := os.Lstat(fullpath)
	if err != nil {
		return nil, convertPathError(err, cleanpath)
	}

	fi = PathFileInfo{st, fullpath}
	// we set sum to path by default for the case where GetFile returns nil.
	// The usual case is if cleanpath is empty.
	sum := path
	if tsInfo := c.sums.GetFile(cleanpath); tsInfo != nil {
		sum = tsInfo.Sum()
	}
	fi = &HashedFileInfo{fi, sum}
	return fi, nil
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
	_, err = os.Stat(fullpath)
	if err != nil {
		return "", "", convertPathError(err, path)
	}
	return
}

func (c *tarSumContext) Walk(root string, walkFn WalkFunc) error {
	for _, tsInfo := range c.sums {
		path := tsInfo.Name()
		path, fullpath, err := c.normalize(path)
		if err != nil {
			return err
		}

		// Any file in the context that starts with the given path will be
		// picked up and its hashcode used.  However, we'll exclude the
		// root dir itself.  We do this for a coupel of reasons:
		// 1 - ADD/COPY will not copy the dir itself, just its children
		//     so there's no reason to include it in the hash calc
		// 2 - the metadata on the dir will change when any child file
		//     changes.  This will lead to a miss in the cache check if that
		//     child file is in the .dockerignore list.
		if rel, err := filepath.Rel(root, path); err != nil {
			return err
		} else if rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			continue
		}

		info, err := os.Lstat(fullpath)
		if err != nil {
			return convertPathError(err, path)
		}
		// TODO check context breakout?
		fi := &HashedFileInfo{PathFileInfo{info, fullpath}, tsInfo.Sum()}
		if err := walkFn(path, fi, nil); err != nil {
			return err
		}
	}
	return nil
}

func (c *tarSumContext) Remove(path string) error {
	_, fullpath, err := c.normalize(path)
	if err != nil {
		return err
	}
	return os.RemoveAll(fullpath)
}
