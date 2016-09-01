// Copyright 2016 The Linux Foundation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package image

import (
	"archive/tar"
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

var (
	errEOW = fmt.Errorf("end of walk") // error to signal stop walking
)

// walkFunc is a function type that gets called for each file or directory visited by the Walker.
type walkFunc func(path string, _ os.FileInfo, _ io.Reader) error

// walker is the interface that defines how to access a given archival format
type walker interface {

	// walk calls walkfunc for every entity in the archive
	walk(walkFunc) error

	// get will copy an arbitrary blob, defined by desc, in to dst. returns
	// the number of bytes copied on success.
	get(desc v1.Descriptor, dst io.Writer) (int64, error)
}

// tarWalker exposes access to image layouts in a tar file.
type tarWalker struct {
	r io.ReadSeeker

	// Synchronize use of the reader
	mut sync.Mutex
}

// newTarWalker returns a Walker that walks through .tar files.
func newTarWalker(r io.ReadSeeker) walker {
	return &tarWalker{r: r}
}

func (w *tarWalker) walk(f walkFunc) error {
	w.mut.Lock()
	defer w.mut.Unlock()

	if _, err := w.r.Seek(0, io.SeekStart); err != nil {
		return errors.Wrapf(err, "unable to reset")
	}

	tr := tar.NewReader(w.r)

loop:
	for {
		hdr, err := tr.Next()
		switch err {
		case io.EOF:
			break loop
		case nil:
			// success, continue below
		default:
			return errors.Wrapf(err, "error advancing tar stream")
		}

		info := hdr.FileInfo()
		if err := f(hdr.Name, info, tr); err != nil {
			return err
		}
	}

	return nil
}

func (w *tarWalker) get(desc v1.Descriptor, dst io.Writer) (int64, error) {
	var bytes int64
	done := false

	expectedPath := filepath.Join("blobs", string(desc.Digest.Algorithm()), desc.Digest.Hex())

	f := func(path string, info os.FileInfo, rdr io.Reader) error {
		var err error
		if done {
			return nil
		}

		if filepath.Clean(path) == expectedPath && !info.IsDir() {
			if bytes, err = io.Copy(dst, rdr); err != nil {
				return errors.Wrapf(err, "get failed: failed to copy blob to destination")
			}
			done = true
		}
		return nil
	}

	if err := w.walk(f); err != nil {
		return 0, errors.Wrapf(err, "get failed: unable to walk")
	}
	if !done {
		return 0, os.ErrNotExist
	}

	return bytes, nil
}

type eofReader struct{}

func (eofReader) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

type pathWalker struct {
	root string
}

// newPathWalker returns a Walker that walks through directories
// starting at the given root path. It does not follow symlinks.
func newPathWalker(root string) walker {
	return &pathWalker{root}
}

func (w *pathWalker) walk(f walkFunc) error {
	return filepath.Walk(w.root, func(path string, info os.FileInfo, err error) error {
		// MUST check error value, to make sure the `os.FileInfo` is available.
		// Otherwise panic risk will exist.
		if err != nil {
			return errors.Wrap(err, "error walking path")
		}

		rel, err := filepath.Rel(w.root, path)
		if err != nil {
			return errors.Wrap(err, "error walking path") // err from filepath.Walk includes path name
		}

		if info.IsDir() { // behave like a tar reader for directories
			return f(rel, info, eofReader{})
		}

		file, err := os.Open(path)
		if err != nil {
			return errors.Wrap(err, "unable to open file") // os.Open includes the path
		}
		defer file.Close()

		return f(rel, info, file)
	})
}

func (w *pathWalker) get(desc v1.Descriptor, dst io.Writer) (int64, error) {
	name := filepath.Join(w.root, "blobs", string(desc.Digest.Algorithm()), desc.Digest.Hex())

	info, err := os.Stat(name)
	if err != nil {
		return 0, err
	}

	if info.IsDir() {
		return 0, fmt.Errorf("object is dir")
	}

	fp, err := os.Open(name)
	if err != nil {
		return 0, errors.Wrapf(err, "get failed")
	}
	defer fp.Close()

	nbytes, err := io.Copy(dst, fp)
	if err != nil {
		return 0, errors.Wrapf(err, "get failed: failed to copy blob to destination")
	}
	return nbytes, nil
}

type zipWalker struct {
	fileName string
}

// newWalkWalker returns a Walker that walks through .zip files.
func newZipWalker(fileName string) walker {
	return &zipWalker{fileName}
}

func (w *zipWalker) walk(f walkFunc) error {
	r, err := zip.OpenReader(w.fileName)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, file := range r.File {
		rc, err := file.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		info := file.FileInfo()
		if err := f(file.Name, info, rc); err != nil {
			return err
		}
	}

	return nil
}

func (w *zipWalker) get(desc v1.Descriptor, dst io.Writer) (int64, error) {
	var bytes int64
	done := false

	expectedPath := filepath.Join("blobs", string(desc.Digest.Algorithm()), desc.Digest.Hex())

	f := func(path string, info os.FileInfo, rdr io.Reader) error {
		var err error
		if done {
			return nil
		}

		if path == expectedPath && !info.IsDir() {
			if bytes, err = io.Copy(dst, rdr); err != nil {
				return errors.Wrapf(err, "get failed: failed to copy blob to destination")
			}
			done = true
		}
		return nil
	}

	if err := w.walk(f); err != nil {
		return 0, errors.Wrapf(err, "get failed: unable to walk")
	}
	if !done {
		return 0, os.ErrNotExist
	}

	return bytes, nil
}
