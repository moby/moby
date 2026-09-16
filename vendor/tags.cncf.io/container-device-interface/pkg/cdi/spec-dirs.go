/*
   Copyright © 2021 The CDI Authors

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package cdi

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	// DefaultStaticDir is the default directory for static CDI Specs.
	DefaultStaticDir = "/etc/cdi"
	// DefaultDynamicDir is the default directory for generated CDI Specs
	DefaultDynamicDir = "/var/run/cdi"
)

// DefaultSpecDirs is the default Spec directory configuration.
//
// The preferred way of overriding the default directories is
// to use [WithSpecDirs], otherwise the change is only effective
// if it takes place before creating the cache instance.
var DefaultSpecDirs = []string{DefaultStaticDir, DefaultDynamicDir}

// ErrStopScan can be returned from a scanSpecFunc to stop the scan.
//
// Deprecated: ErrStopScan was only used by internal scan callbacks and is no longer used.
var ErrStopScan = errors.New("stop Spec scan")

// WithSpecDirs returns an option to override the CDI Spec directories.
func WithSpecDirs(dirs ...string) Option {
	// If no spec dirs are specified use the default spec dirs.
	if len(dirs) == 0 {
		return WithSpecDirs(DefaultSpecDirs...)
	}
	return func(c *Cache) {
		specDirs := make([]string, len(dirs))
		for i, dir := range dirs {
			specDirs[i] = filepath.Clean(dir)
		}
		c.specDirs = specDirs
	}
}

// scanSpecFunc is a function for processing CDI Spec files.
type scanSpecFunc func(string, int, *Spec, error) error

// ScanSpecDirs scans the given directories looking for CDI Spec files,
// which are all files with a '.json' or '.yaml' suffix. For every Spec
// file discovered, ScanSpecDirs loads a Spec from the file then calls
// the scan function passing it the path to the file, the priority (the
// index of the directory in the slice of directories given), the Spec
// itself, and any error encountered while loading the Spec.
//
// Scanning stops once all files have been processed or when the scan
// function returns an error. The result of ScanSpecDirs is the error
// returned by the scan function, if any. ScanSpecDirs does not recurse
// into subdirectories.
func scanSpecDirs(dirs []string, scanFn scanSpecFunc) error {
	for priority, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			// ignore obviously non-Spec files
			if ext := filepath.Ext(entry.Name()); ext != ".json" && ext != ".yaml" {
				continue
			}

			path := filepath.Join(dir, entry.Name())
			spec, specErr := ReadSpec(path, priority) // ignore specErr; it's recorded through scanFn
			if err := scanFn(path, priority, spec, specErr); err != nil {
				return err
			}
		}
	}

	return nil
}
