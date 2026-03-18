// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.BSD file.

// This code is a modified version of path/filepath/symlink.go from the Go
// standard library in [docker@fa3ec89], which was based on [go1.3.3],
// with Windows implementatinos being added in [docker@9b648df].
//
// [docker@fa3ec89]: https://github.com/moby/moby/commit/fa3ec89515431ce425f924c8a9a804d5cb18382f
// [go1.3.3]: https://github.com/golang/go/blob/go1.3.3/src/pkg/path/filepath/symlink.go
// [docker@9b648df]: https://github.com/moby/moby/commit/9b648dfac6453de5944ee4bb749115d85a253a05

package symlink

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// FollowSymlinkInScope evaluates symbolic links in "path" within a scope "root"
// and returns a result guaranteed to be contained within the scope "root" at
// the time of the call. It returns an error of either "path" or "root" cannot
// be converted to an absolute path.
//
// Symbolic links in "root" are not evaluated and left as-is. Errors encountered
// while attempting to evaluate symlinks in path are returned, but non-existing
// paths are valid and do not constitute an error. "path" must contain "root"
// as a prefix, or else an error is returned. Trying to break out from "root"
// does not constitute an error, instead resolves the path within "root".
//
// Example:
//
//	// If "/foo/bar" is a symbolic link to "/outside":
//	FollowSymlinkInScope("/foo/bar", "/foo") // Returns "/foo/outside" instead of "/outside"
//
// IMPORTANT: It is the caller's responsibility to call FollowSymlinkInScope
// after relevant symbolic links are created to avoid Time-of-check Time-of-use
// (TOCTOU) race conditions ([CWE-367]). No additional symbolic links must be
// created after evaluating, as those could potentially make a previously-safe
// path unsafe.
//
// For example, if "/foo/bar" does not exist, FollowSymlinkInScope("/foo/bar", "/foo")
// evaluates the path to "/foo/bar". If one makes "/foo/bar" a symbolic link to
// "/baz" subsequently, then "/foo/bar" should no longer be considered safely
// contained in "/foo".
//
// [CWE-367]: https://cwe.mitre.org/data/definitions/367.html
func FollowSymlinkInScope(path, root string) (string, error) {
	path, err := filepath.Abs(filepath.FromSlash(path))
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(filepath.FromSlash(root))
	if err != nil {
		return "", err
	}
	return evalSymlinksInScope(path, root)
}

// evalSymlinksInScope evaluates symbolic links in "path" within a scope "root"
// and returns a result guaranteed to be contained within the scope "root" at
// the time of the call. Refer to [FollowSymlinkInScope] for details.
func evalSymlinksInScope(path, root string) (string, error) {
	root = filepath.Clean(root)
	if path == root {
		return path, nil
	}
	if !strings.HasPrefix(path, root) {
		return "", errors.New("evalSymlinksInScope: " + path + " is not in " + root)
	}
	const maxIter = 255
	originalPath := path
	// given root of "/a" and path of "/a/b/../../c" we want path to be "/b/../../c"
	path = path[len(root):]
	if root == string(filepath.Separator) {
		path = string(filepath.Separator) + path
	}
	if !strings.HasPrefix(path, string(filepath.Separator)) {
		return "", errors.New("evalSymlinksInScope: " + path + " is not in " + root)
	}
	path = filepath.Clean(path)
	// consume path by taking each frontmost path element,
	// expanding it if it's a symlink, and appending it to b
	var b bytes.Buffer
	// b here will always be considered to be the "current absolute path inside
	// root" when we append paths to it, we also append a slash and use
	// filepath.Clean after the loop to trim the trailing slash
	for n := 0; path != ""; n++ {
		if n > maxIter {
			return "", errors.New("evalSymlinksInScope: too many links in " + originalPath)
		}

		// find next path component, p
		i := strings.IndexRune(path, filepath.Separator)
		var p string
		if i == -1 {
			p, path = path, ""
		} else {
			p, path = path[:i], path[i+1:]
		}

		if p == "" {
			continue
		}

		// this takes a b.String() like "b/../" and a p like "c" and turns it
		// into "/b/../c" which then gets filepath.Cleaned into "/c" and then
		// root gets prepended and we Clean again (to remove any trailing slash
		// if the first Clean gave us just "/")
		cleanP := filepath.Clean(string(filepath.Separator) + b.String() + p)
		if isDriveOrRoot(cleanP) {
			// never Lstat "/" itself, or drive letters on Windows
			b.Reset()
			continue
		}
		fullP := filepath.Clean(root + cleanP)

		fi, err := os.Lstat(fullP)
		if os.IsNotExist(err) {
			// if p does not exist, accept it
			b.WriteString(p)
			b.WriteRune(filepath.Separator)
			continue
		}
		if err != nil {
			return "", err
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			b.WriteString(p)
			b.WriteRune(filepath.Separator)
			continue
		}

		// it's a symlink, put it at the front of path
		dest, err := os.Readlink(fullP)
		if err != nil {
			return "", err
		}
		if isAbs(dest) {
			b.Reset()
		}
		path = dest + string(filepath.Separator) + path
	}

	// see note above on "fullP := ..." for why this is double-cleaned and
	// what's happening here
	return filepath.Clean(root + filepath.Clean(string(filepath.Separator)+b.String())), nil
}

// EvalSymlinks is a modified version of [path/filepath.EvalSymlinks] from
// the Go standard library with support for Windows long paths (paths prepended
// with "\\?\"). On non-Windows platforms, it's an alias for [path/filepath.EvalSymlinks].
//
// EvalSymlinks returns the path name after the evaluation of any symbolic
// links. If path is relative, the result will be relative to the current
// directory, unless one of the components is an absolute symbolic link.
//
// EvalSymlinks calls [path/filepath.Clean] on the result.
func EvalSymlinks(path string) (string, error) {
	return evalSymlinks(path)
}
