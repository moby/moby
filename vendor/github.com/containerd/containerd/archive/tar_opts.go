/*
   Copyright The containerd Authors.

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

package archive

import "archive/tar"

// ApplyOpt allows setting mutable archive apply properties on creation
type ApplyOpt func(options *ApplyOptions) error

// Filter specific files from the archive
type Filter func(*tar.Header) (bool, error)

// all allows all files
func all(_ *tar.Header) (bool, error) {
	return true, nil
}

// WithFilter uses the filter to select which files are to be extracted.
func WithFilter(f Filter) ApplyOpt {
	return func(options *ApplyOptions) error {
		options.Filter = f
		return nil
	}
}
