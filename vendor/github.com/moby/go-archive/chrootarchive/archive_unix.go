//go:build !windows

package chrootarchive

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/go-archive"
	"github.com/moby/go-archive/compression"
	"github.com/moby/sys/user"
)

// Handler for teasing out the automatic decompression
func untarHandler(tarArchive io.Reader, dest string, options *archive.TarOptions, decompress bool, root string) error {
	if tarArchive == nil {
		return errors.New("empty archive")
	}
	if options == nil {
		options = &archive.TarOptions{}
	}

	// Create dest here only if it is the root itself; paths below the root are
	// created by the extractor after entering the chroot.
	// This case is only currently used by cp.
	if dest == root {
		uid, gid := options.IDMap.RootPair()

		dest = filepath.Clean(dest)
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			if err := user.MkdirAllAndChown(dest, 0o755, uid, gid, user.WithOnlyNew); err != nil {
				return err
			}
		}
	}

	r := io.NopCloser(tarArchive)
	if decompress {
		decompressedArchive, err := compression.DecompressStream(tarArchive)
		if err != nil {
			return err
		}
		defer decompressedArchive.Close()
		r = decompressedArchive
	}

	return invokeUnpack(r, dest, options, root)
}

func invokeUnpack(decompressedArchive io.Reader, dest string, options *archive.TarOptions, root string) error {
	relDest, err := resolvePathInChroot(root, dest)
	if err != nil {
		return err
	}

	return doUnpack(decompressedArchive, relDest, root, options)
}

func invokePack(srcPath string, options *archive.TarOptions, root string) (io.ReadCloser, error) {
	relSrc, err := resolvePathInChroot(root, srcPath)
	if err != nil {
		return nil, err
	}

	// make sure we didn't trim a trailing slash with the call to `resolvePathInChroot`
	if strings.HasSuffix(srcPath, "/") && !strings.HasSuffix(relSrc, "/") {
		relSrc += "/"
	}

	return doPack(relSrc, root, options)
}

// resolvePathInChroot returns the equivalent to path inside a chroot rooted at root.
// The returned path always begins with '/'.
//
//   - resolvePathInChroot("/a/b", "/a/b/c/d") -> "/c/d"
//   - resolvePathInChroot("/a/b", "/a/b")     -> "/"
//
// The implementation is buggy, and some bugs may be load-bearing.
// Here be dragons.
func resolvePathInChroot(root, path string) (string, error) {
	if root == "" {
		return "", errors.New("root path must not be empty")
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if rel == "." {
		rel = "/"
	}
	if rel[0] != '/' {
		rel = "/" + rel
	}
	return rel, nil
}
