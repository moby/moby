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

package fsview

import (
	"errors"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"syscall"
)

// OverlayOpaqueXattrs are the xattr names used to indicate an opaque directory.
// "trusted.overlay.opaque" is the traditional xattr used by overlay.
// "user.overlay.opaque" is available since Linux 5.11.
// See https://github.com/torvalds/linux/commit/2d2f2d7322ff43e0fe92bf8cccdc0b09449bf2e1
var OverlayOpaqueXattrs = []string{
	"trusted.overlay.opaque",
	"user.overlay.opaque",
}

// maxSymlinks is the maximum number of symlinks that will be followed
// when resolving a path, to prevent infinite loops.
const maxSymlinks = 255

// NewOverlayFS returns a new fs.FS that overlays the provided layers.
// The layers should be provided in order from upper to lower.
func NewOverlayFS(layers []fs.FS) (fs.FS, error) {
	return &overlayFS{layers: layers}, nil
}

type overlayFS struct {
	layers []fs.FS
}

// hasOpaqueParent checks if any parent directory of the given path is opaque in the layer.
// If a parent is opaque, it means we should not look in lower layers for this path.
func hasOpaqueParent(layer fs.FS, name string) bool {
	// Check all parent directories
	p := name
	for p != "." && p != "/" {
		p = path.Dir(p)
		f, err := layer.Open(p)
		if err != nil {
			continue
		}
		opaque := isOpaque(f)
		f.Close()
		if opaque {
			return true
		}
	}
	return false
}

// lstatLayer returns the FileInfo for name in the layer without following
// the final symlink component. If the layer implements fs.ReadLinkFS, it
// uses Lstat directly. Otherwise it falls back to Open+Stat which follows
// symlinks (degraded behavior).
func lstatLayer(layer fs.FS, name string) (fs.FileInfo, error) {
	if rl, ok := layer.(fs.ReadLinkFS); ok {
		return rl.Lstat(name)
	}
	f, err := layer.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Stat()
}

// readlinkLayer returns the symlink target for name in the layer.
// The layer must implement fs.ReadLinkFS.
func readlinkLayer(layer fs.FS, name string) (string, error) {
	if rl, ok := layer.(fs.ReadLinkFS); ok {
		return rl.ReadLink(name)
	}
	return "", &fs.PathError{Op: "readlink", Path: name, Err: syscall.EINVAL}
}

func (o *overlayFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	return o.openFollow(name, 0)
}

// Lstat returns a FileInfo describing the named file without following
// the final symlink component. Intermediate symlinks are resolved through
// the overlay so that cross-layer symlink targets are found correctly.
func (o *overlayFS) Lstat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: fs.ErrInvalid}
	}
	resolved, err := o.resolve(name, false, 0)
	if err != nil {
		return nil, err
	}
	return o.lstatDirect(resolved)
}

// ReadLink returns the destination of the named symbolic link.
// Intermediate path components are resolved through the overlay.
func (o *overlayFS) ReadLink(name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}
	resolved, err := o.resolve(name, false, 0)
	if err != nil {
		return "", err
	}
	return o.readlinkDirect(resolved)
}

// openFollow opens a file, resolving symlinks at the overlay level.
// depth tracks the number of symlinks followed to detect loops.
func (o *overlayFS) openFollow(name string, depth int) (fs.File, error) {
	resolved, err := o.resolve(name, true, depth)
	if err != nil {
		return nil, err
	}
	return o.openDirect(resolved)
}

// resolve walks the path component by component, resolving symlinks at the
// overlay level. When follow is true, symlinks in the final component are
// also resolved. Returns the fully resolved path with no symlinks.
func (o *overlayFS) resolve(name string, follow bool, depth int) (string, error) {
	if name == "." {
		return ".", nil
	}
	parts := strings.Split(name, "/")
	resolved := ""

	for i, part := range parts {
		isLast := i == len(parts)-1

		var candidate string
		if resolved == "" {
			candidate = part
		} else {
			candidate = resolved + "/" + part
		}

		// Lstat this component across the overlay layers
		fi, err := o.lstatDirect(candidate)
		if err != nil {
			return "", err
		}

		if fi.Mode()&fs.ModeSymlink != 0 {
			if isLast && !follow {
				// Don't follow the final component for Lstat/ReadLink
				resolved = candidate
				continue
			}

			depth++
			if depth > maxSymlinks {
				return "", &fs.PathError{Op: "open", Path: name, Err: syscall.ELOOP}
			}

			target, err := o.readlinkDirect(candidate)
			if err != nil {
				return "", err
			}

			// Build the new path: target + remaining components
			remaining := ""
			if !isLast {
				remaining = strings.Join(parts[i+1:], "/")
			}

			var newPath string
			if path.IsAbs(target) {
				// Absolute symlink: resolve from root
				target = strings.TrimPrefix(target, "/")
				if remaining != "" {
					newPath = target + "/" + remaining
				} else {
					newPath = target
				}
			} else {
				// Relative symlink: resolve from parent of current component
				parent := path.Dir(candidate)
				joined := path.Join(parent, target)
				if remaining != "" {
					newPath = joined + "/" + remaining
				} else {
					newPath = joined
				}
			}
			newPath = path.Clean(newPath)
			if newPath == "." {
				return ".", nil
			}

			// Restart resolution from the root of the overlay
			return o.resolve(newPath, follow, depth)
		}

		if !fi.IsDir() && !isLast {
			// Non-directory, non-symlink in an intermediate position
			return "", &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}

		resolved = candidate
	}

	return resolved, nil
}

// lstatDirect does an Lstat across overlay layers, respecting whiteouts
// and opaque directories. It does NOT resolve symlinks - the path must
// already have intermediate symlinks resolved.
func (o *overlayFS) lstatDirect(name string) (fs.FileInfo, error) {
	var firstErr error
	var opaque bool

	for _, layer := range o.layers {
		if opaque {
			break
		}
		if hasOpaqueParent(layer, name) {
			opaque = true
		}

		fi, err := lstatLayer(layer, name)
		if err != nil {
			var pe *fs.PathError
			if !errors.As(err, &pe) && firstErr == nil {
				firstErr = err
			}
			continue
		}

		if isWhiteout(fi) {
			return nil, &fs.PathError{Op: "lstat", Path: name, Err: fs.ErrNotExist}
		}

		return fi, nil
	}

	if firstErr != nil {
		return nil, firstErr
	}
	return nil, &fs.PathError{Op: "lstat", Path: name, Err: fs.ErrNotExist}
}

// readlinkDirect reads the symlink target from the first matching layer,
// respecting whiteouts and opaque directories. The path must already have
// intermediate symlinks resolved.
func (o *overlayFS) readlinkDirect(name string) (string, error) {
	var opaque bool

	for _, layer := range o.layers {
		if opaque {
			break
		}
		if hasOpaqueParent(layer, name) {
			opaque = true
		}

		fi, err := lstatLayer(layer, name)
		if err != nil {
			continue
		}

		if isWhiteout(fi) {
			return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrNotExist}
		}

		if fi.Mode()&fs.ModeSymlink == 0 {
			return "", &fs.PathError{Op: "readlink", Path: name, Err: syscall.EINVAL}
		}

		return readlinkLayer(layer, name)
	}

	return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrNotExist}
}

// openDirect opens a fully-resolved path (no symlinks) using the
// original layer-by-layer logic with directory merging.
func (o *overlayFS) openDirect(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	var dirLayers []fs.FS
	var firstErr error
	var opaque bool

	for _, layer := range o.layers {
		if opaque {
			// A parent directory in a higher layer is opaque, so we stop looking in lower layers
			break
		}
		if hasOpaqueParent(layer, name) {
			// Set opaqueness but continue to check this layer
			opaque = true
		}

		f, err := layer.Open(name)
		if err != nil {
			// Path errors (not found, not a directory, etc.) are expected
			// when a path doesn't resolve in a given layer. Only record
			// non-path errors (e.g., I/O failures) for later reporting.
			var pe *fs.PathError
			if !errors.As(err, &pe) && firstErr == nil {
				firstErr = err
			}
			continue
		}

		fi, errStat := f.Stat()
		if errStat != nil {
			f.Close()
			if firstErr == nil {
				firstErr = errStat
			}
			continue
		}

		if isWhiteout(fi) {
			f.Close()
			// A whiteout hides this path in all lower layers.
			// If we already found directories above, stop merging.
			if len(dirLayers) > 0 {
				break
			}
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}

		if !fi.IsDir() {
			if len(dirLayers) > 0 {
				// Directory on upper covers file on lower
				f.Close()
				break
			}
			return f, nil
		}

		// Directory — accumulate for merging
		dirLayers = append(dirLayers, layer)
		if !opaque && isOpaque(f) {
			opaque = true
		}
		f.Close()
	}

	if len(dirLayers) > 0 {
		return &overlayDir{fs: o, path: name, layers: dirLayers}, nil
	}

	if firstErr != nil {
		return nil, firstErr
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

type overlayDir struct {
	fs      *overlayFS
	path    string
	layers  []fs.FS
	entries []fs.DirEntry
	offset  int
	read    bool
}

func (d *overlayDir) Stat() (fs.FileInfo, error) {
	// Stat should return info from the top-most layer
	if len(d.layers) == 0 {
		return nil, fs.ErrNotExist
	}
	f, err := d.layers[0].Open(d.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Stat()
}

func (d *overlayDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.path, Err: errors.New("is a directory")}
}

func (d *overlayDir) Close() error {
	return nil
}

func (d *overlayDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if !d.read {
		seen := make(map[string]bool)

		for _, layer := range d.layers {
			// ReadDir from the layer
			entries, err := fs.ReadDir(layer, d.path)
			if err == nil {
				for _, e := range entries {
					name := e.Name()
					if seen[name] {
						continue
					}

					// Check for whiteout in this layer
					if (e.Type() & fs.ModeCharDevice) != 0 {
						info, err := e.Info()
						if err == nil && isWhiteout(info) {
							seen[name] = true
							continue
						}
					}

					seen[name] = true
					d.entries = append(d.entries, e)
				}
			}
		}
		sort.Slice(d.entries, func(i, j int) bool { return d.entries[i].Name() < d.entries[j].Name() })
		d.read = true
	}

	if n <= 0 {
		if d.offset >= len(d.entries) {
			return []fs.DirEntry{}, nil
		}
		res := d.entries[d.offset:]
		d.offset = len(d.entries)
		return res, nil
	}

	if d.offset >= len(d.entries) {
		return nil, io.EOF
	}

	end := min(d.offset+n, len(d.entries))
	res := d.entries[d.offset:end]
	d.offset = end
	return res, nil
}
