//go:build linux
// +build linux

package overlay

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/containerd/containerd/archive"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/continuity/devices"
	"github.com/containerd/continuity/fs"
	"github.com/containerd/continuity/sysx"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// GetUpperdir parses the passed mounts and identifies the directory
// that contains diff between upper and lower.
func GetUpperdir(lower, upper []mount.Mount) (string, error) {
	var upperdir string
	if len(lower) == 0 && len(upper) == 1 { // upper is the bottommost snapshot
		// Get layer directories of upper snapshot
		upperM := upper[0]
		if upperM.Type != "bind" {
			return "", errors.Errorf("bottommost upper must be bind mount but %q", upperM.Type)
		}
		upperdir = upperM.Source
	} else if len(lower) == 1 && len(upper) == 1 {
		// Get layer directories of lower snapshot
		var lowerlayers []string
		lowerM := lower[0]
		switch lowerM.Type {
		case "bind":
			// lower snapshot is a bind mount of one layer
			lowerlayers = []string{lowerM.Source}
		case "overlay":
			// lower snapshot is an overlay mount of multiple layers
			var err error
			lowerlayers, err = GetOverlayLayers(lowerM)
			if err != nil {
				return "", err
			}
		default:
			return "", errors.Errorf("cannot get layer information from mount option (type = %q)", lowerM.Type)
		}

		// Get layer directories of upper snapshot
		upperM := upper[0]
		if upperM.Type != "overlay" {
			return "", errors.Errorf("upper snapshot isn't overlay mounted (type = %q)", upperM.Type)
		}
		upperlayers, err := GetOverlayLayers(upperM)
		if err != nil {
			return "", err
		}

		// Check if the diff directory can be determined
		if len(upperlayers) != len(lowerlayers)+1 {
			return "", errors.Errorf("cannot determine diff of more than one upper directories")
		}
		for i := 0; i < len(lowerlayers); i++ {
			if upperlayers[i] != lowerlayers[i] {
				return "", errors.Errorf("layer %d must be common between upper and lower snapshots", i)
			}
		}
		upperdir = upperlayers[len(upperlayers)-1] // get the topmost layer that indicates diff
	} else {
		return "", errors.Errorf("multiple mount configurations are not supported")
	}
	if upperdir == "" {
		return "", errors.Errorf("cannot determine upperdir from mount option")
	}
	return upperdir, nil
}

// GetOverlayLayers returns all layer directories of an overlayfs mount.
func GetOverlayLayers(m mount.Mount) ([]string, error) {
	var u string
	var uFound bool
	var l []string // l[0] = bottommost
	for _, o := range m.Options {
		if strings.HasPrefix(o, "upperdir=") {
			u, uFound = strings.TrimPrefix(o, "upperdir="), true
		} else if strings.HasPrefix(o, "lowerdir=") {
			l = strings.Split(strings.TrimPrefix(o, "lowerdir="), ":")
			for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
				l[i], l[j] = l[j], l[i] // make l[0] = bottommost
			}
		} else if strings.HasPrefix(o, "workdir=") || o == "index=off" || o == "userxattr" || strings.HasPrefix(o, "redirect_dir=") {
			// these options are possible to specfied by the snapshotter but not indicate dir locations.
			continue
		} else {
			// encountering an unknown option. return error and fallback to walking differ
			// to avoid unexpected diff.
			return nil, errors.Errorf("unknown option %q specified by snapshotter", o)
		}
	}
	if uFound {
		return append(l, u), nil
	}
	return l, nil
}

// WriteUpperdir writes a layer tar archive into the specified writer, based on
// the diff information stored in the upperdir.
func WriteUpperdir(ctx context.Context, w io.Writer, upperdir string, lower []mount.Mount) error {
	emptyLower, err := os.MkdirTemp("", "buildkit") // empty directory used for the lower of diff view
	if err != nil {
		return errors.Wrapf(err, "failed to create temp dir")
	}
	defer os.Remove(emptyLower)
	upperView := []mount.Mount{
		{
			Type:    "overlay",
			Source:  "overlay",
			Options: []string{fmt.Sprintf("lowerdir=%s", strings.Join([]string{upperdir, emptyLower}, ":"))},
		},
	}
	return mount.WithTempMount(ctx, lower, func(lowerRoot string) error {
		return mount.WithTempMount(ctx, upperView, func(upperViewRoot string) error {
			cw := archive.NewChangeWriter(&cancellableWriter{ctx, w}, upperViewRoot)
			if err := Changes(ctx, cw.HandleChange, upperdir, upperViewRoot, lowerRoot); err != nil {
				if err2 := cw.Close(); err2 != nil {
					return errors.Wrapf(err, "failed to record upperdir changes (close error: %v)", err2)
				}
				return errors.Wrapf(err, "failed to record upperdir changes")
			}
			return cw.Close()
		})
	})
}

type cancellableWriter struct {
	ctx context.Context
	w   io.Writer
}

func (w *cancellableWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	return w.w.Write(p)
}

// Changes is continuty's `fs.Change`-like method but leverages overlayfs's
// "upperdir" for computing the diff. "upperdirView" is overlayfs mounted view of
// the upperdir that doesn't contain whiteouts. This is used for computing
// changes under opaque directories.
func Changes(ctx context.Context, changeFn fs.ChangeFunc, upperdir, upperdirView, base string) error {
	return filepath.Walk(upperdir, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Rebase path
		path, err = filepath.Rel(upperdir, path)
		if err != nil {
			return err
		}
		path = filepath.Join(string(os.PathSeparator), path)

		// Skip root
		if path == string(os.PathSeparator) {
			return nil
		}

		// Check redirect
		if redirect, err := checkRedirect(upperdir, path, f); err != nil {
			return err
		} else if redirect {
			// Return error when redirect_dir is enabled which can result to a wrong diff.
			// TODO: support redirect_dir
			return fmt.Errorf("redirect_dir is used but it's not supported in overlayfs differ")
		}

		// Check if this is a deleted entry
		isDelete, skip, err := checkDelete(upperdir, path, base, f)
		if err != nil {
			return err
		} else if skip {
			return nil
		}

		var kind fs.ChangeKind
		var skipRecord bool
		if isDelete {
			// This is a deleted entry.
			kind = fs.ChangeKindDelete
			// Leave f set to the FileInfo for the whiteout device in case the caller wants it, e.g.
			// the merge code uses it to hardlink in the whiteout device to merged snapshots
		} else if baseF, err := os.Lstat(filepath.Join(base, path)); err == nil {
			// File exists in the base layer. Thus this is modified.
			kind = fs.ChangeKindModify
			// Avoid including directory that hasn't been modified. If /foo/bar/baz is modified,
			// then /foo will apper here even if it's not been modified because it's the parent of bar.
			if same, err := sameDirent(baseF, f, filepath.Join(base, path), filepath.Join(upperdirView, path)); same {
				skipRecord = true // Both are the same, don't record the change
			} else if err != nil {
				return err
			}
		} else if os.IsNotExist(err) || errors.Is(err, unix.ENOTDIR) {
			// File doesn't exist in the base layer. Thus this is added.
			kind = fs.ChangeKindAdd
		} else if err != nil {
			return errors.Wrap(err, "failed to stat base file during overlay diff")
		}

		if !skipRecord {
			if err := changeFn(kind, path, f, nil); err != nil {
				return err
			}
		}

		if f != nil {
			if isOpaque, err := checkOpaque(upperdir, path, base, f); err != nil {
				return err
			} else if isOpaque {
				// This is an opaque directory. Start a new walking differ to get adds/deletes of
				// this directory. We use "upperdirView" directory which doesn't contain whiteouts.
				if err := fs.Changes(ctx, filepath.Join(base, path), filepath.Join(upperdirView, path),
					func(k fs.ChangeKind, p string, f os.FileInfo, err error) error {
						return changeFn(k, filepath.Join(path, p), f, err) // rebase path to be based on the opaque dir
					},
				); err != nil {
					return err
				}
				return filepath.SkipDir // We completed this directory. Do not walk files under this directory anymore.
			}
		}
		return nil
	})
}

// checkDelete checks if the specified file is a whiteout
func checkDelete(upperdir string, path string, base string, f os.FileInfo) (delete, skip bool, _ error) {
	if f.Mode()&os.ModeCharDevice != 0 {
		if _, ok := f.Sys().(*syscall.Stat_t); ok {
			maj, min, err := devices.DeviceInfo(f)
			if err != nil {
				return false, false, errors.Wrapf(err, "failed to get device info")
			}
			if maj == 0 && min == 0 {
				// This file is a whiteout (char 0/0) that indicates this is deleted from the base
				if _, err := os.Lstat(filepath.Join(base, path)); err != nil {
					if !os.IsNotExist(err) {
						return false, false, errors.Wrapf(err, "failed to lstat")
					}
					// This file doesn't exist even in the base dir.
					// We don't need whiteout. Just skip this file.
					return false, true, nil
				}
				return true, false, nil
			}
		}
	}
	return false, false, nil
}

// checkDelete checks if the specified file is an opaque directory
func checkOpaque(upperdir string, path string, base string, f os.FileInfo) (isOpaque bool, _ error) {
	if f.IsDir() {
		for _, oKey := range []string{"trusted.overlay.opaque", "user.overlay.opaque"} {
			opaque, err := sysx.LGetxattr(filepath.Join(upperdir, path), oKey)
			if err != nil && err != unix.ENODATA {
				return false, errors.Wrapf(err, "failed to retrieve %s attr", oKey)
			} else if len(opaque) == 1 && opaque[0] == 'y' {
				// This is an opaque whiteout directory.
				if _, err := os.Lstat(filepath.Join(base, path)); err != nil {
					if !os.IsNotExist(err) {
						return false, errors.Wrapf(err, "failed to lstat")
					}
					// This file doesn't exist even in the base dir. We don't need treat this as an opaque.
					return false, nil
				}
				return true, nil
			}
		}
	}
	return false, nil
}

// checkRedirect checks if the specified path enables redirect_dir.
func checkRedirect(upperdir string, path string, f os.FileInfo) (bool, error) {
	if f.IsDir() {
		rKey := "trusted.overlay.redirect"
		redirect, err := sysx.LGetxattr(filepath.Join(upperdir, path), rKey)
		if err != nil && err != unix.ENODATA {
			return false, errors.Wrapf(err, "failed to retrieve %s attr", rKey)
		}
		return len(redirect) > 0, nil
	}
	return false, nil
}

// sameDirent performs continity-compatible comparison of files and directories.
// https://github.com/containerd/continuity/blob/v0.1.0/fs/path.go#L91-L133
// This will only do a slow content comparison of two files if they have all the
// same metadata and both have truncated nanosecond mtime timestamps. In practice,
// this can only happen if both the base file in the lowerdirs has a truncated
// timestamp (i.e. was unpacked from a tar) and the user did something like
// "mv foo tmp && mv tmp foo" that results in the file being copied up to the
// upperdir without making any changes to it. This is much rarer than similar
// cases in the double-walking differ, where the slow content comparison will
// be used whenever a file with a truncated timestamp is in the lowerdir at
// all and left unmodified.
func sameDirent(f1, f2 os.FileInfo, f1fullPath, f2fullPath string) (bool, error) {
	if os.SameFile(f1, f2) {
		return true, nil
	}

	equalStat, err := compareSysStat(f1.Sys(), f2.Sys())
	if err != nil || !equalStat {
		return equalStat, err
	}

	if eq, err := compareCapabilities(f1fullPath, f2fullPath); err != nil || !eq {
		return eq, err
	}

	if !f1.IsDir() {
		if f1.Size() != f2.Size() {
			return false, nil
		}
		t1 := f1.ModTime()
		t2 := f2.ModTime()

		if t1.Unix() != t2.Unix() {
			return false, nil
		}

		// If the timestamp may have been truncated in both of the
		// files, check content of file to determine difference
		if t1.Nanosecond() == 0 && t2.Nanosecond() == 0 {
			if (f1.Mode() & os.ModeSymlink) == os.ModeSymlink {
				return compareSymlinkTarget(f1fullPath, f2fullPath)
			}
			if f1.Size() == 0 {
				return true, nil
			}
			return compareFileContent(f1fullPath, f2fullPath)
		} else if t1.Nanosecond() != t2.Nanosecond() {
			return false, nil
		}
	}

	return true, nil
}

// Ported from continuity project
// https://github.com/containerd/continuity/blob/v0.1.0/fs/diff_unix.go#L43-L54
// Copyright The containerd Authors.
func compareSysStat(s1, s2 interface{}) (bool, error) {
	ls1, ok := s1.(*syscall.Stat_t)
	if !ok {
		return false, nil
	}
	ls2, ok := s2.(*syscall.Stat_t)
	if !ok {
		return false, nil
	}

	return ls1.Mode == ls2.Mode && ls1.Uid == ls2.Uid && ls1.Gid == ls2.Gid && ls1.Rdev == ls2.Rdev, nil
}

// Ported from continuity project
// https://github.com/containerd/continuity/blob/v0.1.0/fs/diff_unix.go#L56-L66
// Copyright The containerd Authors.
func compareCapabilities(p1, p2 string) (bool, error) {
	c1, err := sysx.LGetxattr(p1, "security.capability")
	if err != nil && err != sysx.ENODATA {
		return false, errors.Wrapf(err, "failed to get xattr for %s", p1)
	}
	c2, err := sysx.LGetxattr(p2, "security.capability")
	if err != nil && err != sysx.ENODATA {
		return false, errors.Wrapf(err, "failed to get xattr for %s", p2)
	}
	return bytes.Equal(c1, c2), nil
}

// Ported from continuity project
// https://github.com/containerd/continuity/blob/bce1c3f9669b6f3e7f6656ee715b0b4d75fa64a6/fs/path.go#L135
// Copyright The containerd Authors.
func compareSymlinkTarget(p1, p2 string) (bool, error) {
	t1, err := os.Readlink(p1)
	if err != nil {
		return false, err
	}
	t2, err := os.Readlink(p2)
	if err != nil {
		return false, err
	}
	return t1 == t2, nil
}

var bufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 32*1024)
		return &b
	},
}

// Ported from continuity project
// https://github.com/containerd/continuity/blob/bce1c3f9669b6f3e7f6656ee715b0b4d75fa64a6/fs/path.go#L151
// Copyright The containerd Authors.
func compareFileContent(p1, p2 string) (bool, error) {
	f1, err := os.Open(p1)
	if err != nil {
		return false, err
	}
	defer f1.Close()
	if stat, err := f1.Stat(); err != nil {
		return false, err
	} else if !stat.Mode().IsRegular() {
		return false, errors.Errorf("%s is not a regular file", p1)
	}

	f2, err := os.Open(p2)
	if err != nil {
		return false, err
	}
	defer f2.Close()
	if stat, err := f2.Stat(); err != nil {
		return false, err
	} else if !stat.Mode().IsRegular() {
		return false, errors.Errorf("%s is not a regular file", p2)
	}

	b1 := bufPool.Get().(*[]byte)
	defer bufPool.Put(b1)
	b2 := bufPool.Get().(*[]byte)
	defer bufPool.Put(b2)
	for {
		n1, err1 := io.ReadFull(f1, *b1)
		if err1 == io.ErrUnexpectedEOF {
			// it's expected to get EOF when file size isn't a multiple of chunk size, consolidate these error types
			err1 = io.EOF
		}
		if err1 != nil && err1 != io.EOF {
			return false, err1
		}
		n2, err2 := io.ReadFull(f2, *b2)
		if err2 == io.ErrUnexpectedEOF {
			err2 = io.EOF
		}
		if err2 != nil && err2 != io.EOF {
			return false, err2
		}
		if n1 != n2 || !bytes.Equal((*b1)[:n1], (*b2)[:n2]) {
			return false, nil
		}
		if err1 == io.EOF && err2 == io.EOF {
			return true, nil
		}
	}
}
