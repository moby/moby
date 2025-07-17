package safepath

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// evaluatePath evaluates symlinks in the concatenation of path and subpath. If
// err is nil, resolvedBasePath will contain result of resolving all symlinks
// in the given path, and resolvedSubpath will contain a relative path rooted
// at the resolvedBasePath pointing to the concatenation after resolving all
// symlinks.
func evaluatePath(path, subpath string) (resolvedBasePath string, resolvedSubpath string, err error) {
	baseResolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", &ErrNotAccessible{Path: path, Cause: err}
		}
		return "", "", errors.Wrapf(err, "error while resolving symlinks in base directory %q", path)
	}

	combinedPath := filepath.Join(baseResolved, subpath)
	combinedResolved, err := filepath.EvalSymlinks(combinedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", &ErrNotAccessible{Path: combinedPath, Cause: err}
		}
		return "", "", errors.Wrapf(err, "error while resolving symlinks in combined path %q", combinedPath)
	}

	subpart, err := filepath.Rel(baseResolved, combinedResolved)
	if err != nil {
		return "", "", &ErrEscapesBase{Base: baseResolved, Subpath: subpath}
	}

	if !filepath.IsLocal(subpart) {
		return "", "", &ErrEscapesBase{Base: baseResolved, Subpath: subpath}
	}

	return baseResolved, subpart, nil
}

// isLocalTo reports whether path, using lexical analysis only, has all of these properties:
//   - is within the subtree rooted at basepath
//   - is not empty
//   - on Windows, is not a reserved name such as "NUL"
//
// If isLocalTo(path, basepath) returns true, then
//
//	filepath.Rel(basepath, path)
//
// will always produce an unrooted path with no `..` elements.
//
// isLocalTo is a purely lexical operation. In particular, it does not account for the effect of any symbolic links that may exist in the filesystem.
//
// Both path and basepath are expected to be absolute paths.
func isLocalTo(path, basepath string) bool {
	rel, err := filepath.Rel(basepath, path)
	if err != nil {
		return false
	}

	return filepath.IsLocal(rel)
}
