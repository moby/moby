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

package manager

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/containerd/log"

	"github.com/containerd/containerd/v2/core/mount"
	"github.com/docker/go-units"
)

type mkfs struct {
	rootMap map[string]*os.Root
}

func (t *mkfs) Transform(ctx context.Context, m mount.Mount, a []mount.ActiveMount) (mount.Mount, error) {
	var r *os.Root
	var subpath string

	for path, root := range t.rootMap {
		if strings.HasPrefix(m.Source, path) {
			r = root
			subpath = strings.TrimPrefix(m.Source, path)
			subpath, _ = filepath.Rel("/", subpath)
			break
		}
	}
	if r == nil {
		err := fmt.Errorf("no root %q configured for mkfs: %w", m.Source, errdefs.ErrNotImplemented)
		log.G(ctx).WithError(err).Debugf("skipping mkfs")
		return m, err
	}

	log.G(ctx).Debugf("transforming mkfs mount: %+v", m)

	var (
		size int64
		id   string
		fs   = "ext4"
	)
	var options []string
	for _, o := range m.Options {
		if mkfsOption, isMkfs := strings.CutPrefix(o, prefixMkfs); isMkfs {
			key, value, ok := strings.Cut(mkfsOption, "=")
			if !ok {
				key = o
				value = "true"
			}
			switch key {
			case "size":
				var err error
				size, err = units.RAMInBytes(value)
				if err != nil {
					return mount.Mount{}, fmt.Errorf("bad option %s: %w", key, err)
				}
			case "fs":
				fs = value
			case "uuid":
				id = value
			default:
				return mount.Mount{}, fmt.Errorf("unknown mount option %s: %w", key, errdefs.ErrInvalidArgument)
			}

		} else {
			options = append(options, o)
		}
	}
	m.Options = options
	if size == 0 {
		return mount.Mount{}, fmt.Errorf("mkfs requires mkfs.size option: %w", errdefs.ErrInvalidArgument)
	}

	if _, err := r.Stat(subpath); err == nil {
		// Check magic number
	} else if os.IsNotExist(err) {
		createArgs := []string{"-q"}

		// TODO: Pre-resolve the binaries to absolute path on startup for supported fs types
		var binary string

		// Check fs
		switch fs {
		case "ext2", "ext3", "ext4":
			binary = fmt.Sprintf("mkfs.%s", fs)
			if id != "" {
				createArgs = append(createArgs, []string{"-U", id}...)
			}
		case "xfs":
			binary = "mkfs.xfs"
			if id != "" {
				createArgs = append(createArgs, []string{"-m", fmt.Sprintf("uuid=%s", id)}...)
			}
		default:
			return mount.Mount{}, fmt.Errorf("unsupported filesystem %q: %w", fs, errdefs.ErrInvalidArgument)
		}

		f, err := r.OpenFile(subpath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0640)
		if err != nil {
			return mount.Mount{}, fmt.Errorf("failed to create file %q: %w", m.Source, err)
		}

		createArgs = append(createArgs, f.Name())

		err = f.Truncate(size)
		f.Close()
		if err != nil {
			return mount.Mount{}, fmt.Errorf("failed to truncate file %q: %w", m.Source, err)
		}

		if err := createWritableImage(ctx, binary, createArgs...); err != nil {
			return mount.Mount{}, fmt.Errorf("failed format %q: %w", m.Source, err)
		}
	} else {
		return mount.Mount{}, fmt.Errorf("failed to stat %q: %w", m.Source, err)
	}

	return m, nil
}

func createWritableImage(ctx context.Context, binary string, args ...string) error {
	cmd := exec.CommandContext(ctx, binary, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %s: %w", filepath.Base(binary), out, err)
	}
	return nil
}
