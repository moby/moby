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

import (
	"archive/tar"
	"context"
	"io"
)

// ApplyOptions provides additional options for an Apply operation
type ApplyOptions struct {
	Filter          Filter          // Filter tar headers
	ConvertWhiteout ConvertWhiteout // Convert whiteout files
	Parents         []string        // Parent directories to handle inherited attributes without CoW

	applyFunc func(context.Context, string, io.Reader, ApplyOptions) (int64, error)
}

// ApplyOpt allows setting mutable archive apply properties on creation
type ApplyOpt func(options *ApplyOptions) error

// Filter specific files from the archive
type Filter func(*tar.Header) (bool, error)

// ConvertWhiteout converts whiteout files from the archive
type ConvertWhiteout func(*tar.Header, string) (bool, error)

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

// WithConvertWhiteout uses the convert function to convert the whiteout files.
func WithConvertWhiteout(c ConvertWhiteout) ApplyOpt {
	return func(options *ApplyOptions) error {
		options.ConvertWhiteout = c
		return nil
	}
}

// WithParents provides parent directories for resolving inherited attributes
// directory from the filesystem.
// Inherited attributes are searched from first to last, making the first
// element in the list the most immediate parent directory.
// NOTE: When applying to a filesystem which supports CoW, file attributes
// should be inherited by the filesystem.
func WithParents(p []string) ApplyOpt {
	return func(options *ApplyOptions) error {
		options.Parents = p
		return nil
	}
}

// WriteDiffOptions provides additional options for a WriteDiff operation
type WriteDiffOptions struct {
	ParentLayers []string // Windows needs the full list of parent layers

	writeDiffFunc func(context.Context, io.Writer, string, string, WriteDiffOptions) error
}

// WriteDiffOpt allows setting mutable archive write properties on creation
type WriteDiffOpt func(options *WriteDiffOptions) error
