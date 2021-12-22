// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gopathwalk is like filepath.Walk but specialized for finding Go
// packages, particularly in $GOPATH and $GOROOT.
package gopathwalk

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/tools/internal/fastwalk"
)

// Options controls the behavior of a Walk call.
type Options struct {
	// If Logf is non-nil, debug logging is enabled through this function.
	Logf func(format string, args ...interface{})
	// Search module caches. Also disables legacy goimports ignore rules.
	ModulesEnabled bool
}

// RootType indicates the type of a Root.
type RootType int

const (
	RootUnknown RootType = iota
	RootGOROOT
	RootGOPATH
	RootCurrentModule
	RootModuleCache
	RootOther
)

// A Root is a starting point for a Walk.
type Root struct {
	Path string
	Type RootType
}

// Walk walks Go source directories ($GOROOT, $GOPATH, etc) to find packages.
// For each package found, add will be called (concurrently) with the absolute
// paths of the containing source directory and the package directory.
// add will be called concurrently.
func Walk(roots []Root, add func(root Root, dir string), opts Options) {
	WalkSkip(roots, add, func(Root, string) bool { return false }, opts)
}

// WalkSkip walks Go source directories ($GOROOT, $GOPATH, etc) to find packages.
// For each package found, add will be called (concurrently) with the absolute
// paths of the containing source directory and the package directory.
// For each directory that will be scanned, skip will be called (concurrently)
// with the absolute paths of the containing source directory and the directory.
// If skip returns false on a directory it will be processed.
// add will be called concurrently.
// skip will be called concurrently.
func WalkSkip(roots []Root, add func(root Root, dir string), skip func(root Root, dir string) bool, opts Options) {
	for _, root := range roots {
		walkDir(root, add, skip, opts)
	}
}

// walkDir creates a walker and starts fastwalk with this walker.
func walkDir(root Root, add func(Root, string), skip func(root Root, dir string) bool, opts Options) {
	if _, err := os.Stat(root.Path); os.IsNotExist(err) {
		if opts.Logf != nil {
			opts.Logf("skipping nonexistent directory: %v", root.Path)
		}
		return
	}
	start := time.Now()
	if opts.Logf != nil {
		opts.Logf("gopathwalk: scanning %s", root.Path)
	}
	w := &walker{
		root: root,
		add:  add,
		skip: skip,
		opts: opts,
	}
	w.init()
	if err := fastwalk.Walk(root.Path, w.walk); err != nil {
		log.Printf("gopathwalk: scanning directory %v: %v", root.Path, err)
	}

	if opts.Logf != nil {
		opts.Logf("gopathwalk: scanned %s in %v", root.Path, time.Since(start))
	}
}

// walker is the callback for fastwalk.Walk.
type walker struct {
	root Root                    // The source directory to scan.
	add  func(Root, string)      // The callback that will be invoked for every possible Go package dir.
	skip func(Root, string) bool // The callback that will be invoked for every dir. dir is skipped if it returns true.
	opts Options                 // Options passed to Walk by the user.

	ignoredDirs []os.FileInfo // The ignored directories, loaded from .goimportsignore files.
}

// init initializes the walker based on its Options
func (w *walker) init() {
	var ignoredPaths []string
	if w.root.Type == RootModuleCache {
		ignoredPaths = []string{"cache"}
	}
	if !w.opts.ModulesEnabled && w.root.Type == RootGOPATH {
		ignoredPaths = w.getIgnoredDirs(w.root.Path)
		ignoredPaths = append(ignoredPaths, "v", "mod")
	}

	for _, p := range ignoredPaths {
		full := filepath.Join(w.root.Path, p)
		if fi, err := os.Stat(full); err == nil {
			w.ignoredDirs = append(w.ignoredDirs, fi)
			if w.opts.Logf != nil {
				w.opts.Logf("Directory added to ignore list: %s", full)
			}
		} else if w.opts.Logf != nil {
			w.opts.Logf("Error statting ignored directory: %v", err)
		}
	}
}

// getIgnoredDirs reads an optional config file at <path>/.goimportsignore
// of relative directories to ignore when scanning for go files.
// The provided path is one of the $GOPATH entries with "src" appended.
func (w *walker) getIgnoredDirs(path string) []string {
	file := filepath.Join(path, ".goimportsignore")
	slurp, err := ioutil.ReadFile(file)
	if w.opts.Logf != nil {
		if err != nil {
			w.opts.Logf("%v", err)
		} else {
			w.opts.Logf("Read %s", file)
		}
	}
	if err != nil {
		return nil
	}

	var ignoredDirs []string
	bs := bufio.NewScanner(bytes.NewReader(slurp))
	for bs.Scan() {
		line := strings.TrimSpace(bs.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ignoredDirs = append(ignoredDirs, line)
	}
	return ignoredDirs
}

// shouldSkipDir reports whether the file should be skipped or not.
func (w *walker) shouldSkipDir(fi os.FileInfo, dir string) bool {
	for _, ignoredDir := range w.ignoredDirs {
		if os.SameFile(fi, ignoredDir) {
			return true
		}
	}
	if w.skip != nil {
		// Check with the user specified callback.
		return w.skip(w.root, dir)
	}
	return false
}

// walk walks through the given path.
func (w *walker) walk(path string, typ os.FileMode) error {
	dir := filepath.Dir(path)
	if typ.IsRegular() {
		if dir == w.root.Path && (w.root.Type == RootGOROOT || w.root.Type == RootGOPATH) {
			// Doesn't make sense to have regular files
			// directly in your $GOPATH/src or $GOROOT/src.
			return fastwalk.ErrSkipFiles
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		w.add(w.root, dir)
		return fastwalk.ErrSkipFiles
	}
	if typ == os.ModeDir {
		base := filepath.Base(path)
		if base == "" || base[0] == '.' || base[0] == '_' ||
			base == "testdata" ||
			(w.root.Type == RootGOROOT && w.opts.ModulesEnabled && base == "vendor") ||
			(!w.opts.ModulesEnabled && base == "node_modules") {
			return filepath.SkipDir
		}
		fi, err := os.Lstat(path)
		if err == nil && w.shouldSkipDir(fi, path) {
			return filepath.SkipDir
		}
		return nil
	}
	if typ == os.ModeSymlink {
		base := filepath.Base(path)
		if strings.HasPrefix(base, ".#") {
			// Emacs noise.
			return nil
		}
		fi, err := os.Lstat(path)
		if err != nil {
			// Just ignore it.
			return nil
		}
		if w.shouldTraverse(dir, fi) {
			return fastwalk.ErrTraverseLink
		}
	}
	return nil
}

// shouldTraverse reports whether the symlink fi, found in dir,
// should be followed.  It makes sure symlinks were never visited
// before to avoid symlink loops.
func (w *walker) shouldTraverse(dir string, fi os.FileInfo) bool {
	path := filepath.Join(dir, fi.Name())
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	ts, err := os.Stat(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return false
	}
	if !ts.IsDir() {
		return false
	}
	if w.shouldSkipDir(ts, dir) {
		return false
	}
	// Check for symlink loops by statting each directory component
	// and seeing if any are the same file as ts.
	for {
		parent := filepath.Dir(path)
		if parent == path {
			// Made it to the root without seeing a cycle.
			// Use this symlink.
			return true
		}
		parentInfo, err := os.Stat(parent)
		if err != nil {
			return false
		}
		if os.SameFile(ts, parentInfo) {
			// Cycle. Don't traverse.
			return false
		}
		path = parent
	}

}
