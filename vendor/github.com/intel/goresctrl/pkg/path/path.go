/*
Copyright 2023 Intel Corporation

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

package path

import "path/filepath"

// RootDir is a helper for handling system directory paths
type RootDir string

var prefix RootDir = "/"

// Path returns a full path to a file under RootDir
func (d RootDir) Path(elems ...string) string {
	return filepath.Join(append([]string{string(d)}, elems...)...)
}

// SetPrefix sets the global path prefix to use for all system files.
func SetPrefix(p string) { prefix = RootDir(p) }

// Path returns a path to a file, prefixed with the global prefix.
func Path(elems ...string) string { return prefix.Path(elems...) }
