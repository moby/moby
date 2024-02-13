//go:build !linux
// +build !linux

/*
   Copyright © 2022 The CDI Authors

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
	"os"
	"path/filepath"
)

// Rename src to dst, both relative to the directory dir. If dst already exists
// refuse renaming with an error unless overwrite is explicitly asked for.
func renameIn(dir, src, dst string, overwrite bool) error {
	src = filepath.Join(dir, src)
	dst = filepath.Join(dir, dst)

	_, err := os.Stat(dst)
	if err == nil && !overwrite {
		return os.ErrExist
	}

	return os.Rename(src, dst)
}
