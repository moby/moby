//go:build linux
// +build linux

package cache

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/containerd/containerd/archive"
	ctdcompression "github.com/containerd/containerd/archive/compression"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/continuity/devices"
	"github.com/containerd/continuity/fs"
	"github.com/containerd/continuity/sysx"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

var emptyDesc = ocispecs.Descriptor{}

// computeOverlayBlob provides overlayfs-specialized method to compute
// diff between lower and upper snapshot. If the passed mounts cannot
// be computed (e.g. because the mounts aren't overlayfs), it returns
// an error.
func (sr *immutableRef) tryComputeOverlayBlob(ctx context.Context, lower, upper []mount.Mount, mediaType string, ref string, compressorFunc compressor) (_ ocispecs.Descriptor, ok bool, err error) {
	// Get upperdir location if mounts are overlayfs that can be processed by this differ.
	upperdir, err := getOverlayUpperdir(lower, upper)
	if err != nil {
		// This is not an overlayfs snapshot. This is not an error so don't return error here
		// and let the caller fallback to another differ.
		return emptyDesc, false, nil
	}

	if compressorFunc == nil {
		switch mediaType {
		case ocispecs.MediaTypeImageLayer:
		case ocispecs.MediaTypeImageLayerGzip:
			compressorFunc = func(dest io.Writer, requiredMediaType string) (io.WriteCloser, error) {
				return ctdcompression.CompressStream(dest, ctdcompression.Gzip)
			}
		case ocispecs.MediaTypeImageLayer + "+zstd":
			compressorFunc = zstdWriter
		default:
			return emptyDesc, false, errors.Errorf("unsupported diff media type: %v", mediaType)
		}
	}

	cw, err := sr.cm.ContentStore.Writer(ctx,
		content.WithRef(ref),
		content.WithDescriptor(ocispecs.Descriptor{
			MediaType: mediaType, // most contentstore implementations just ignore this
		}))
	if err != nil {
		return emptyDesc, false, errors.Wrap(err, "failed to open writer")
	}
	defer func() {
		if err != nil {
			cw.Close()
		}
	}()

	bufW := bufio.NewWriterSize(cw, 128*1024)
	var labels map[string]string
	if compressorFunc != nil {
		dgstr := digest.SHA256.Digester()
		compressed, err := compressorFunc(bufW, mediaType)
		if err != nil {
			return emptyDesc, false, errors.Wrap(err, "failed to get compressed stream")
		}
		err = writeOverlayUpperdir(ctx, io.MultiWriter(compressed, dgstr.Hash()), upperdir, lower)
		compressed.Close()
		if err != nil {
			return emptyDesc, false, errors.Wrap(err, "failed to write compressed diff")
		}
		if labels == nil {
			labels = map[string]string{}
		}
		labels[containerdUncompressed] = dgstr.Digest().String()
	} else {
		if err = writeOverlayUpperdir(ctx, bufW, upperdir, lower); err != nil {
			return emptyDesc, false, errors.Wrap(err, "failed to write diff")
		}
	}

	if err := bufW.Flush(); err != nil {
		return emptyDesc, false, errors.Wrap(err, "failed to flush diff")
	}
	var commitopts []content.Opt
	if labels != nil {
		commitopts = append(commitopts, content.WithLabels(labels))
	}
	dgst := cw.Digest()
	if err := cw.Commit(ctx, 0, dgst, commitopts...); err != nil {
		if !errdefs.IsAlreadyExists(err) {
			return emptyDesc, false, errors.Wrap(err, "failed to commit")
		}
	}
	cinfo, err := sr.cm.ContentStore.Info(ctx, dgst)
	if err != nil {
		return emptyDesc, false, errors.Wrap(err, "failed to get info from content store")
	}
	if cinfo.Labels == nil {
		cinfo.Labels = make(map[string]string)
	}
	// Set uncompressed label if digest already existed without label
	if _, ok := cinfo.Labels[containerdUncompressed]; !ok {
		cinfo.Labels[containerdUncompressed] = labels[containerdUncompressed]
		if _, err := sr.cm.ContentStore.Update(ctx, cinfo, "labels."+containerdUncompressed); err != nil {
			return emptyDesc, false, errors.Wrap(err, "error setting uncompressed label")
		}
	}

	return ocispecs.Descriptor{
		MediaType: mediaType,
		Size:      cinfo.Size,
		Digest:    cinfo.Digest,
	}, true, nil
}

// getOverlayUpperdir parses the passed mounts and identifies the directory
// that contains diff between upper and lower.
func getOverlayUpperdir(lower, upper []mount.Mount) (string, error) {
	var upperdir string
	if len(lower) == 0 && len(upper) == 1 { // upper is the bottommost snpashot
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
			lowerlayers, err = getOverlayLayers(lowerM)
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
		upperlayers, err := getOverlayLayers(upperM)
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

// getOverlayLayers returns all layer directories of an overlayfs mount.
func getOverlayLayers(m mount.Mount) ([]string, error) {
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
		} else if strings.HasPrefix(o, "workdir=") || o == "index=off" || o == "userxattr" {
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

// writeOverlayUpperdir writes a layer tar archive into the specified writer, based on
// the diff information stored in the upperdir.
func writeOverlayUpperdir(ctx context.Context, w io.Writer, upperdir string, lower []mount.Mount) error {
	emptyLower, err := ioutil.TempDir("", "buildkit") // empty directory used for the lower of diff view
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
			cw := archive.NewChangeWriter(w, upperViewRoot)
			if err := overlayChanges(ctx, cw.HandleChange, upperdir, upperViewRoot, lowerRoot); err != nil {
				if err2 := cw.Close(); err2 != nil {
					return errors.Wrapf(err, "failed torecord upperdir changes (close error: %v)", err2)
				}
				return errors.Wrapf(err, "failed torecord upperdir changes")
			}
			return cw.Close()
		})
	})
}

// overlayChanges is continuty's `fs.Change`-like method but leverages overlayfs's
// "upperdir" for computing the diff. "upperdirView" is overlayfs mounted view of
// the upperdir that doesn't contain whiteouts. This is used for computing
// changes under opaque directories.
func overlayChanges(ctx context.Context, changeFn fs.ChangeFunc, upperdir, upperdirView, base string) error {
	return filepath.Walk(upperdir, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
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
			f = nil
		} else if baseF, err := os.Lstat(filepath.Join(base, path)); err == nil {
			// File exists in the base layer. Thus this is modified.
			kind = fs.ChangeKindModify
			// Avoid including directory that hasn't been modified. If /foo/bar/baz is modified,
			// then /foo will apper here even if it's not been modified because it's the parent of bar.
			if same, err := sameDir(baseF, f, filepath.Join(base, path), filepath.Join(upperdirView, path)); same {
				skipRecord = true // Both are the same, don't record the change
			} else if err != nil {
				return err
			}
		} else if os.IsNotExist(err) {
			// File doesn't exist in the base layer. Thus this is added.
			kind = fs.ChangeKindAdd
		} else if err != nil {
			return err
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

// sameDir performs continity-compatible comparison of directories.
// https://github.com/containerd/continuity/blob/v0.1.0/fs/path.go#L91-L133
// This doesn't compare files because it requires to compare their contents.
// This is what we want to avoid by this overlayfs-specialized differ.
func sameDir(f1, f2 os.FileInfo, f1fullPath, f2fullPath string) (bool, error) {
	if !f1.IsDir() || !f2.IsDir() {
		return false, nil
	}

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
