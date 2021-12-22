// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package buildutil

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ParseFile behaves like parser.ParseFile,
// but uses the build context's file system interface, if any.
//
// If file is not absolute (as defined by IsAbsPath), the (dir, file)
// components are joined using JoinPath; dir must be absolute.
//
// The displayPath function, if provided, is used to transform the
// filename that will be attached to the ASTs.
//
// TODO(adonovan): call this from go/loader.parseFiles when the tree thaws.
//
func ParseFile(fset *token.FileSet, ctxt *build.Context, displayPath func(string) string, dir string, file string, mode parser.Mode) (*ast.File, error) {
	if !IsAbsPath(ctxt, file) {
		file = JoinPath(ctxt, dir, file)
	}
	rd, err := OpenFile(ctxt, file)
	if err != nil {
		return nil, err
	}
	defer rd.Close() // ignore error
	if displayPath != nil {
		file = displayPath(file)
	}
	return parser.ParseFile(fset, file, rd, mode)
}

// ContainingPackage returns the package containing filename.
//
// If filename is not absolute, it is interpreted relative to working directory dir.
// All I/O is via the build context's file system interface, if any.
//
// The '...Files []string' fields of the resulting build.Package are not
// populated (build.FindOnly mode).
//
func ContainingPackage(ctxt *build.Context, dir, filename string) (*build.Package, error) {
	if !IsAbsPath(ctxt, filename) {
		filename = JoinPath(ctxt, dir, filename)
	}

	// We must not assume the file tree uses
	// "/" always,
	// `\` always,
	// or os.PathSeparator (which varies by platform),
	// but to make any progress, we are forced to assume that
	// paths will not use `\` unless the PathSeparator
	// is also `\`, thus we can rely on filepath.ToSlash for some sanity.

	dirSlash := path.Dir(filepath.ToSlash(filename)) + "/"

	// We assume that no source root (GOPATH[i] or GOROOT) contains any other.
	for _, srcdir := range ctxt.SrcDirs() {
		srcdirSlash := filepath.ToSlash(srcdir) + "/"
		if importPath, ok := HasSubdir(ctxt, srcdirSlash, dirSlash); ok {
			return ctxt.Import(importPath, dir, build.FindOnly)
		}
	}

	return nil, fmt.Errorf("can't find package containing %s", filename)
}

// -- Effective methods of file system interface -------------------------

// (go/build.Context defines these as methods, but does not export them.)

// hasSubdir calls ctxt.HasSubdir (if not nil) or else uses
// the local file system to answer the question.
func HasSubdir(ctxt *build.Context, root, dir string) (rel string, ok bool) {
	if f := ctxt.HasSubdir; f != nil {
		return f(root, dir)
	}

	// Try using paths we received.
	if rel, ok = hasSubdir(root, dir); ok {
		return
	}

	// Try expanding symlinks and comparing
	// expanded against unexpanded and
	// expanded against expanded.
	rootSym, _ := filepath.EvalSymlinks(root)
	dirSym, _ := filepath.EvalSymlinks(dir)

	if rel, ok = hasSubdir(rootSym, dir); ok {
		return
	}
	if rel, ok = hasSubdir(root, dirSym); ok {
		return
	}
	return hasSubdir(rootSym, dirSym)
}

func hasSubdir(root, dir string) (rel string, ok bool) {
	const sep = string(filepath.Separator)
	root = filepath.Clean(root)
	if !strings.HasSuffix(root, sep) {
		root += sep
	}

	dir = filepath.Clean(dir)
	if !strings.HasPrefix(dir, root) {
		return "", false
	}

	return filepath.ToSlash(dir[len(root):]), true
}

// FileExists returns true if the specified file exists,
// using the build context's file system interface.
func FileExists(ctxt *build.Context, path string) bool {
	if ctxt.OpenFile != nil {
		r, err := ctxt.OpenFile(path)
		if err != nil {
			return false
		}
		r.Close() // ignore error
		return true
	}
	_, err := os.Stat(path)
	return err == nil
}

// OpenFile behaves like os.Open,
// but uses the build context's file system interface, if any.
func OpenFile(ctxt *build.Context, path string) (io.ReadCloser, error) {
	if ctxt.OpenFile != nil {
		return ctxt.OpenFile(path)
	}
	return os.Open(path)
}

// IsAbsPath behaves like filepath.IsAbs,
// but uses the build context's file system interface, if any.
func IsAbsPath(ctxt *build.Context, path string) bool {
	if ctxt.IsAbsPath != nil {
		return ctxt.IsAbsPath(path)
	}
	return filepath.IsAbs(path)
}

// JoinPath behaves like filepath.Join,
// but uses the build context's file system interface, if any.
func JoinPath(ctxt *build.Context, path ...string) string {
	if ctxt.JoinPath != nil {
		return ctxt.JoinPath(path...)
	}
	return filepath.Join(path...)
}

// IsDir behaves like os.Stat plus IsDir,
// but uses the build context's file system interface, if any.
func IsDir(ctxt *build.Context, path string) bool {
	if ctxt.IsDir != nil {
		return ctxt.IsDir(path)
	}
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// ReadDir behaves like ioutil.ReadDir,
// but uses the build context's file system interface, if any.
func ReadDir(ctxt *build.Context, path string) ([]os.FileInfo, error) {
	if ctxt.ReadDir != nil {
		return ctxt.ReadDir(path)
	}
	return ioutil.ReadDir(path)
}

// SplitPathList behaves like filepath.SplitList,
// but uses the build context's file system interface, if any.
func SplitPathList(ctxt *build.Context, s string) []string {
	if ctxt.SplitPathList != nil {
		return ctxt.SplitPathList(s)
	}
	return filepath.SplitList(s)
}

// sameFile returns true if x and y have the same basename and denote
// the same file.
//
func sameFile(x, y string) bool {
	if path.Clean(x) == path.Clean(y) {
		return true
	}
	if filepath.Base(x) == filepath.Base(y) { // (optimisation)
		if xi, err := os.Stat(x); err == nil {
			if yi, err := os.Stat(y); err == nil {
				return os.SameFile(xi, yi)
			}
		}
	}
	return false
}
