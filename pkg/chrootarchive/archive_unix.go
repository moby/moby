//go:build !windows
// +build !windows

package chrootarchive // import "github.com/docker/docker/pkg/chrootarchive"

import (
	"io"
	"net"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/archive"
	"github.com/pkg/errors"
)

func init() {
	// initialize nss libraries in Glibc so that the dynamic libraries are loaded in the host
	// environment not in the chroot from untrusted files.
	_, _ = user.Lookup("docker")
	_, _ = net.LookupHost("localhost")
}

func invokeUnpack(decompressedArchive io.Reader, dest string, options *archive.TarOptions, root string) error {
	relDest, err := resolvePathInChroot(root, dest)
	if err != nil {
		return err
	}

	done := make(chan error)
	err = goInChroot(root, func() { done <- archive.Unpack(decompressedArchive, relDest, options) })
	if err != nil {
		return err
	}
	return <-done
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

	tb, err := archive.NewTarballer(relSrc, options)
	if err != nil {
		return nil, errors.Wrap(err, "error processing tar file")
	}
	err = goInChroot(root, tb.Do)
	if err != nil {
		return nil, errors.Wrap(err, "could not chroot")
	}
	return tb.Reader(), nil
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
