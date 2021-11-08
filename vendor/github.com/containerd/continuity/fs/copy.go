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

package fs

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

var bufferPool = &sync.Pool{
	New: func() interface{} {
		buffer := make([]byte, 32*1024)
		return &buffer
	},
}

// XAttrErrorHandlers transform a non-nil xattr error.
// Return nil to ignore an error.
// xattrKey can be empty for listxattr operation.
type XAttrErrorHandler func(dst, src, xattrKey string, err error) error

type copyDirOpts struct {
	xeh XAttrErrorHandler
	// xex contains a set of xattrs to exclude when copying
	xex map[string]struct{}
}

type CopyDirOpt func(*copyDirOpts) error

// WithXAttrErrorHandler allows specifying XAttrErrorHandler
// If nil XAttrErrorHandler is specified (default), CopyDir stops
// on a non-nil xattr error.
func WithXAttrErrorHandler(xeh XAttrErrorHandler) CopyDirOpt {
	return func(o *copyDirOpts) error {
		o.xeh = xeh
		return nil
	}
}

// WithAllowXAttrErrors allows ignoring xattr errors.
func WithAllowXAttrErrors() CopyDirOpt {
	xeh := func(dst, src, xattrKey string, err error) error {
		return nil
	}
	return WithXAttrErrorHandler(xeh)
}

// WithXAttrExclude allows for exclusion of specified xattr during CopyDir operation.
func WithXAttrExclude(keys ...string) CopyDirOpt {
	return func(o *copyDirOpts) error {
		if o.xex == nil {
			o.xex = make(map[string]struct{}, len(keys))
		}
		for _, key := range keys {
			o.xex[key] = struct{}{}
		}
		return nil
	}
}

// CopyDir copies the directory from src to dst.
// Most efficient copy of files is attempted.
func CopyDir(dst, src string, opts ...CopyDirOpt) error {
	var o copyDirOpts
	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return err
		}
	}
	inodes := map[uint64]string{}
	return copyDirectory(dst, src, inodes, &o)
}

func copyDirectory(dst, src string, inodes map[uint64]string, o *copyDirOpts) error {
	stat, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", src, err)
	}
	if !stat.IsDir() {
		return fmt.Errorf("source %s is not directory", src)
	}

	if st, err := os.Stat(dst); err != nil {
		if err := os.Mkdir(dst, stat.Mode()); err != nil {
			return fmt.Errorf("failed to mkdir %s: %w", dst, err)
		}
	} else if !st.IsDir() {
		return fmt.Errorf("cannot copy to non-directory: %s", dst)
	} else {
		if err := os.Chmod(dst, stat.Mode()); err != nil {
			return fmt.Errorf("failed to chmod on %s: %w", dst, err)
		}
	}

	fis, err := ioutil.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", src, err)
	}

	if err := copyFileInfo(stat, dst); err != nil {
		return fmt.Errorf("failed to copy file info for %s: %w", dst, err)
	}

	if err := copyXAttrs(dst, src, o.xex, o.xeh); err != nil {
		return fmt.Errorf("failed to copy xattrs: %w", err)
	}

	for _, fi := range fis {
		source := filepath.Join(src, fi.Name())
		target := filepath.Join(dst, fi.Name())

		switch {
		case fi.IsDir():
			if err := copyDirectory(target, source, inodes, o); err != nil {
				return err
			}
			continue
		case (fi.Mode() & os.ModeType) == 0:
			link, err := getLinkSource(target, fi, inodes)
			if err != nil {
				return fmt.Errorf("failed to get hardlink: %w", err)
			}
			if link != "" {
				if err := os.Link(link, target); err != nil {
					return fmt.Errorf("failed to create hard link: %w", err)
				}
			} else if err := CopyFile(target, source); err != nil {
				return fmt.Errorf("failed to copy files: %w", err)
			}
		case (fi.Mode() & os.ModeSymlink) == os.ModeSymlink:
			link, err := os.Readlink(source)
			if err != nil {
				return fmt.Errorf("failed to read link: %s: %w", source, err)
			}
			if err := os.Symlink(link, target); err != nil {
				return fmt.Errorf("failed to create symlink: %s: %w", target, err)
			}
		case (fi.Mode() & os.ModeDevice) == os.ModeDevice:
			if err := copyDevice(target, fi); err != nil {
				return fmt.Errorf("failed to create device: %w", err)
			}
		default:
			// TODO: Support pipes and sockets
			return fmt.Errorf("unsupported mode %s: %w", fi.Mode(), err)
		}
		if err := copyFileInfo(fi, target); err != nil {
			return fmt.Errorf("failed to copy file info: %w", err)
		}

		if err := copyXAttrs(target, source, o.xex, o.xeh); err != nil {
			return fmt.Errorf("failed to copy xattrs: %w", err)
		}
	}

	return nil
}

// CopyFile copies the source file to the target.
// The most efficient means of copying is used for the platform.
func CopyFile(target, source string) error {
	src, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source %s: %w", source, err)
	}
	defer src.Close()
	tgt, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to open target %s: %w", target, err)
	}
	defer tgt.Close()

	return copyFileContent(tgt, src)
}
