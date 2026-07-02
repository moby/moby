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
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containerd/errdefs"

	"github.com/containerd/containerd/v2/core/mount"
)

// mkdir is a mount transformer that creates directories
// it can be used to ensure directories are created before
// overlay options are applied or any case that might need
// a directory
type mkdir struct {
	rootMap map[string]*os.Root
}

func (h *mkdir) Transform(ctx context.Context, m mount.Mount, _ []mount.ActiveMount) (mount.Mount, error) {

	var options []string
	for _, o := range m.Options {
		if mkdirOption, isMkdir := strings.CutPrefix(o, prefixMkdir); isMkdir {
			// Format is X-containerd.mkdir.path=value[:mode[:uid:gid]]

			value, isPath := strings.CutPrefix(mkdirOption, "path=")
			if !isPath {
				return mount.Mount{}, fmt.Errorf("invalid mkdir option %q: %w", o, errdefs.ErrInvalidArgument)
			}
			parts := strings.SplitN(value, ":", 4)
			var (
				dir      string
				mode     os.FileMode = 0700
				luid                 = os.Getuid()
				lgid                 = os.Getgid()
				uid, gid             = luid, lgid
				err      error
			)
			switch len(parts) {
			case 4:
				gid, err = strconv.Atoi(parts[3])
				if err != nil {
					return mount.Mount{}, fmt.Errorf("invalid gid %q: %w", parts[3], errdefs.ErrInvalidArgument)
				}
				uid, err = strconv.Atoi(parts[2])
				if err != nil {
					return mount.Mount{}, fmt.Errorf("invalid uid %q: %w", parts[2], errdefs.ErrInvalidArgument)
				}
				fallthrough
			case 2:
				var p uint64
				p, err = strconv.ParseUint(parts[1], 8, 32)
				if err == nil {
					mode = os.FileMode(p)
					if mode != mode&os.ModePerm {
						return mount.Mount{}, fmt.Errorf("invalid mode %o", p)
					}
				} else {
					return mount.Mount{}, fmt.Errorf("invalid mode %s: %w", parts[1], err)
				}
				fallthrough
			case 1:
				dir = parts[0]
			default:
				return mount.Mount{}, fmt.Errorf("invalid mkdir option %q: %w", o, errdefs.ErrInvalidArgument)
			}

			var r *os.Root
			var subpath string

			for path, root := range h.rootMap {
				if strings.HasPrefix(dir, path) {
					r = root
					subpath = strings.TrimPrefix(dir, path)
					subpath, _ = filepath.Rel("/", subpath)
					break
				}
			}
			if r == nil {
				return mount.Mount{}, fmt.Errorf("no root %q configured for mkdir: %w", dir, errdefs.ErrNotImplemented)
			}

			if st, err := r.Stat(subpath); err == nil {
				if st.Mode()&os.ModePerm != mode {
					// TODO: Chmod support added in go1.25
					return mount.Mount{}, fmt.Errorf("chmod not supported yet for mkdir handler: %w", errdefs.ErrNotImplemented)
				}
				// TODO: check ownership, chown support added in go1.25
			} else if os.IsNotExist(err) {
				// TODO: MkdirAll added in go1.25
				if err := r.Mkdir(subpath, mode); err != nil {
					return mount.Mount{}, fmt.Errorf("failed to create directory %q: %w", dir, err)
				}
				if luid != -1 && (luid != uid || lgid != gid) {
					// TODO: Chown support added in go1.25
					//if err := r.Chown(subpath, uid, gid); err != nil {
					//	return fmt.Errorf("failed to chown directory %q: %w", m.Source, err)
					//}
					return mount.Mount{}, fmt.Errorf("chown not supported yet for mkdir handler: %w", errdefs.ErrNotImplemented)
				}
			} else {
				return mount.Mount{}, fmt.Errorf("failed to stat %q: %w", dir, err)
			}
		} else {
			options = append(options, o)
		}
	}
	m.Options = options

	return m, nil
}
