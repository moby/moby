//go:build !windows
// +build !windows

package snapshot

import (
	"context"
	gofs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/containerd/snapshots/overlay/overlayutils"
	"github.com/containerd/continuity/fs"
	"github.com/containerd/continuity/sysx"
	"github.com/hashicorp/go-multierror"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/util/bklog"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/overlay"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// diffApply applies the provided diffs to the dest Mountable and returns the correctly calculated disk usage
// that accounts for any hardlinks made from existing snapshots. ctx is expected to have a temporary lease
// associated with it.
func (sn *mergeSnapshotter) diffApply(ctx context.Context, dest Mountable, diffs ...Diff) (_ snapshots.Usage, rerr error) {
	a, err := applierFor(dest, sn.tryCrossSnapshotLink, sn.userxattr)
	if err != nil {
		return snapshots.Usage{}, errors.Wrapf(err, "failed to create applier")
	}
	defer func() {
		releaseErr := a.Release()
		if releaseErr != nil {
			rerr = multierror.Append(rerr, errors.Wrapf(releaseErr, "failed to release applier")).ErrorOrNil()
		}
	}()

	// TODO:(sipsma) optimization: parallelize differ and applier in separate goroutines, connected with a buffered channel

	for _, diff := range diffs {
		var lowerMntable Mountable
		if diff.Lower != "" {
			if info, err := sn.Stat(ctx, diff.Lower); err != nil {
				return snapshots.Usage{}, errors.Wrapf(err, "failed to stat lower snapshot %s", diff.Lower)
			} else if info.Kind == snapshots.KindCommitted {
				lowerMntable, err = sn.View(ctx, identity.NewID(), diff.Lower)
				if err != nil {
					return snapshots.Usage{}, errors.Wrapf(err, "failed to mount lower snapshot view %s", diff.Lower)
				}
			} else {
				lowerMntable, err = sn.Mounts(ctx, diff.Lower)
				if err != nil {
					return snapshots.Usage{}, errors.Wrapf(err, "failed to mount lower snapshot %s", diff.Lower)
				}
			}
		}
		var upperMntable Mountable
		if diff.Upper != "" {
			if info, err := sn.Stat(ctx, diff.Upper); err != nil {
				return snapshots.Usage{}, errors.Wrapf(err, "failed to stat upper snapshot %s", diff.Upper)
			} else if info.Kind == snapshots.KindCommitted {
				upperMntable, err = sn.View(ctx, identity.NewID(), diff.Upper)
				if err != nil {
					return snapshots.Usage{}, errors.Wrapf(err, "failed to mount upper snapshot view %s", diff.Upper)
				}
			} else {
				upperMntable, err = sn.Mounts(ctx, diff.Upper)
				if err != nil {
					return snapshots.Usage{}, errors.Wrapf(err, "failed to mount upper snapshot %s", diff.Upper)
				}
			}
		} else {
			// create an empty view
			upperMntable, err = sn.View(ctx, identity.NewID(), "")
			if err != nil {
				return snapshots.Usage{}, errors.Wrapf(err, "failed to mount empty upper snapshot view %s", diff.Upper)
			}
		}
		d, err := differFor(lowerMntable, upperMntable)
		if err != nil {
			return snapshots.Usage{}, errors.Wrapf(err, "failed to create differ")
		}
		defer func() {
			rerr = multierror.Append(rerr, d.Release()).ErrorOrNil()
		}()
		if err := d.HandleChanges(ctx, a.Apply); err != nil {
			return snapshots.Usage{}, errors.Wrapf(err, "failed to handle changes")
		}
	}

	if err := a.Flush(); err != nil {
		return snapshots.Usage{}, errors.Wrapf(err, "failed to flush changes")
	}
	return a.Usage()
}

type change struct {
	kind    fs.ChangeKind
	subPath string
	srcPath string
	srcStat *syscall.Stat_t
	// linkSubPath is set to a subPath of a previous change from the same
	// differ instance that is a hardlink to this one, if any.
	linkSubPath string
}

type changeApply struct {
	*change
	dstPath   string
	dstStat   *syscall.Stat_t
	setOpaque bool
}

type inode struct {
	ino uint64
	dev uint64
}

func statInode(stat *syscall.Stat_t) inode {
	if stat == nil {
		return inode{}
	}
	return inode{
		ino: stat.Ino,
		dev: stat.Dev,
	}
}

type applier struct {
	root                 string
	release              func() error
	lowerdirs            []string // ordered highest -> lowest, the order we want to check them in
	crossSnapshotLinks   map[inode]struct{}
	createWhiteoutDelete bool
	userxattr            bool
	dirModTimes          map[string]unix.Timespec // map of dstPath -> mtime that should be set on that subPath
}

func applierFor(dest Mountable, tryCrossSnapshotLink, userxattr bool) (_ *applier, rerr error) {
	a := &applier{
		dirModTimes: make(map[string]unix.Timespec),
		userxattr:   userxattr,
	}
	defer func() {
		if rerr != nil {
			rerr = multierror.Append(rerr, a.Release()).ErrorOrNil()
		}
	}()
	if tryCrossSnapshotLink {
		a.crossSnapshotLinks = make(map[inode]struct{})
	}

	mnts, release, err := dest.Mount()
	if err != nil {
		return nil, nil
	}
	a.release = release

	if len(mnts) != 1 {
		return nil, errors.Errorf("expected exactly one mount, got %d", len(mnts))
	}
	mnt := mnts[0]

	switch mnt.Type {
	case "overlay":
		for _, opt := range mnt.Options {
			if strings.HasPrefix(opt, "upperdir=") {
				a.root = strings.TrimPrefix(opt, "upperdir=")
			} else if strings.HasPrefix(opt, "lowerdir=") {
				a.lowerdirs = strings.Split(strings.TrimPrefix(opt, "lowerdir="), ":")
			}
		}
		if a.root == "" {
			return nil, errors.Errorf("could not find upperdir in mount options %v", mnt.Options)
		}
		if len(a.lowerdirs) == 0 {
			return nil, errors.Errorf("could not find lowerdir in mount options %v", mnt.Options)
		}
		a.createWhiteoutDelete = true
	case "bind", "rbind":
		a.root = mnt.Source
	default:
		mnter := LocalMounter(dest)
		root, err := mnter.Mount()
		if err != nil {
			return nil, err
		}
		a.root = root
		prevRelease := a.release
		a.release = func() error {
			err := mnter.Unmount()
			return multierror.Append(err, prevRelease()).ErrorOrNil()
		}
	}

	a.root, err = filepath.EvalSymlinks(a.root)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve symlinks in %s", a.root)
	}
	return a, nil
}

func (a *applier) Apply(ctx context.Context, c *change) error {
	if c == nil {
		return errors.New("nil change")
	}

	if c.kind == fs.ChangeKindUnmodified {
		return nil
	}

	dstPath, err := safeJoin(a.root, c.subPath)
	if err != nil {
		return errors.Wrapf(err, "failed to join paths %q and %q", a.root, c.subPath)
	}
	var dstStat *syscall.Stat_t
	if dstfi, err := os.Lstat(dstPath); err == nil {
		stat, ok := dstfi.Sys().(*syscall.Stat_t)
		if !ok {
			return errors.Errorf("failed to get stat_t for %T", dstStat)
		}
		dstStat = stat
	} else if !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to stat during copy apply")
	}

	ca := &changeApply{
		change:  c,
		dstPath: dstPath,
		dstStat: dstStat,
	}

	if done, err := a.applyDelete(ctx, ca); err != nil {
		return errors.Wrap(err, "failed to delete during apply")
	} else if done {
		return nil
	}

	if done, err := a.applyHardlink(ctx, ca); err != nil {
		return errors.Wrapf(err, "failed to hardlink during apply")
	} else if done {
		return nil
	}

	if err := a.applyCopy(ctx, ca); err != nil {
		return errors.Wrapf(err, "failed to copy during apply")
	}
	return nil
}

func (a *applier) applyDelete(ctx context.Context, ca *changeApply) (bool, error) {
	// Even when not deleting, we may be overwriting a file, in which case we should
	// delete the existing file at the path, if any. Don't delete when both are dirs
	// in this case though because they should get merged, not overwritten.
	deleteOnly := ca.kind == fs.ChangeKindDelete
	overwrite := !deleteOnly && ca.dstStat != nil && ca.srcStat.Mode&ca.dstStat.Mode&unix.S_IFMT != unix.S_IFDIR

	if !deleteOnly && !overwrite {
		// nothing to delete, continue on
		return false, nil
	}

	if err := os.RemoveAll(ca.dstPath); err != nil {
		return false, errors.Wrap(err, "failed to remove during apply")
	}
	ca.dstStat = nil

	if overwrite && a.createWhiteoutDelete && ca.srcStat.Mode&unix.S_IFMT == unix.S_IFDIR {
		// If we are using an overlay snapshotter and overwriting an existing non-directory
		// with a directory, we need this new dir to be opaque so that any files from lowerdirs
		// under it are not visible.
		ca.setOpaque = true
	}

	if deleteOnly && a.createWhiteoutDelete {
		// only create a whiteout device if there is something to delete
		var foundLower bool
		for _, lowerdir := range a.lowerdirs {
			lowerPath, err := safeJoin(lowerdir, ca.subPath)
			if err != nil {
				return false, errors.Wrapf(err, "failed to join lowerdir %q and subPath %q", lowerdir, ca.subPath)
			}
			if _, err := os.Lstat(lowerPath); err == nil {
				foundLower = true
				break
			} else if !errors.Is(err, unix.ENOENT) && !errors.Is(err, unix.ENOTDIR) {
				return false, errors.Wrapf(err, "failed to stat lowerPath %q", lowerPath)
			}
		}
		if foundLower {
			ca.kind = fs.ChangeKindAdd
			if ca.srcStat == nil {
				ca.srcStat = &syscall.Stat_t{
					Mode: syscall.S_IFCHR,
					Rdev: unix.Mkdev(0, 0),
				}
				ca.srcPath = ""
			}
			return false, nil
		}
	}

	return deleteOnly, nil
}

func (a *applier) applyHardlink(ctx context.Context, ca *changeApply) (bool, error) {
	switch ca.srcStat.Mode & unix.S_IFMT {
	case unix.S_IFDIR, unix.S_IFIFO, unix.S_IFSOCK:
		// Directories can't be hard-linked, so they just have to be recreated.
		// Named pipes and sockets can be hard-linked but is best to avoid as it could enable IPC in weird cases.
		return false, nil

	default:
		var linkSrcPath string
		if ca.linkSubPath != "" {
			// there's an already applied path that we should link from
			path, err := safeJoin(a.root, ca.linkSubPath)
			if err != nil {
				return false, errors.Errorf("failed to get hardlink source path: %v", err)
			}
			linkSrcPath = path
		} else if a.crossSnapshotLinks != nil {
			// we can try to link across snapshots from the source file
			linkSrcPath = ca.srcPath
			a.crossSnapshotLinks[statInode(ca.srcStat)] = struct{}{}
		}
		if linkSrcPath == "" {
			// nothing to hardlink from, will have to copy the file
			return false, nil
		}

		if err := os.Link(linkSrcPath, ca.dstPath); errors.Is(err, unix.EXDEV) || errors.Is(err, unix.EMLINK) {
			// These errors are expected when the hardlink would cross devices or would exceed the maximum number of links for the inode.
			// Just fallback to a copy.
			bklog.G(ctx).WithError(err).WithField("srcPath", linkSrcPath).WithField("dstPath", ca.dstPath).Debug("hardlink failed")
			if a.crossSnapshotLinks != nil {
				delete(a.crossSnapshotLinks, statInode(ca.srcStat))
			}
			return false, nil
		} else if err != nil {
			return false, errors.Wrap(err, "failed to hardlink during apply")
		}

		return true, nil
	}
}

func (a *applier) applyCopy(ctx context.Context, ca *changeApply) error {
	switch ca.srcStat.Mode & unix.S_IFMT {
	case unix.S_IFREG:
		if err := fs.CopyFile(ca.dstPath, ca.srcPath); err != nil {
			return errors.Wrapf(err, "failed to copy from %s to %s during apply", ca.srcPath, ca.dstPath)
		}
	case unix.S_IFDIR:
		if ca.dstStat == nil {
			// dstPath doesn't exist, make it a dir
			if err := unix.Mkdir(ca.dstPath, ca.srcStat.Mode); err != nil {
				return errors.Wrapf(err, "failed to create applied dir at %q from %q", ca.dstPath, ca.srcPath)
			}
		}
	case unix.S_IFLNK:
		if target, err := os.Readlink(ca.srcPath); err != nil {
			return errors.Wrap(err, "failed to read symlink during apply")
		} else if err := os.Symlink(target, ca.dstPath); err != nil {
			return errors.Wrap(err, "failed to create symlink during apply")
		}
	case unix.S_IFBLK, unix.S_IFCHR, unix.S_IFIFO, unix.S_IFSOCK:
		if err := unix.Mknod(ca.dstPath, ca.srcStat.Mode, int(ca.srcStat.Rdev)); err != nil {
			return errors.Wrap(err, "failed to mknod during apply")
		}
	default:
		// should never be here, all types should be handled
		return errors.Errorf("unhandled file type %d during merge at path %q", ca.srcStat.Mode&unix.S_IFMT, ca.srcPath)
	}

	// NOTE: it's important that chown happens before setting xattrs due to the fact that chown will
	// reset the security.capabilities xattr which results in file capabilities being lost.
	if err := os.Lchown(ca.dstPath, int(ca.srcStat.Uid), int(ca.srcStat.Gid)); err != nil {
		return errors.Wrap(err, "failed to chown during apply")
	}

	if ca.srcStat.Mode&unix.S_IFMT != unix.S_IFLNK {
		if err := unix.Chmod(ca.dstPath, ca.srcStat.Mode); err != nil {
			return errors.Wrapf(err, "failed to chmod path %q during apply", ca.dstPath)
		}
	}

	if ca.srcPath != "" {
		xattrs, err := sysx.LListxattr(ca.srcPath)
		if err != nil {
			return errors.Wrapf(err, "failed to list xattrs of src path %s", ca.srcPath)
		}
		for _, xattr := range xattrs {
			if isOpaqueXattr(xattr) {
				// Don't recreate opaque xattrs during merge based on the source file. The differs take care of converting
				// source path from the "opaque whiteout" format to the "explicit whiteout" format. The only time we set
				// opaque xattrs is handled after this loop below.
				continue
			}
			xattrVal, err := sysx.LGetxattr(ca.srcPath, xattr)
			if err != nil {
				return errors.Wrapf(err, "failed to get xattr %s of src path %s", xattr, ca.srcPath)
			}
			if err := sysx.LSetxattr(ca.dstPath, xattr, xattrVal, 0); err != nil {
				// This can often fail, so just log it: https://github.com/moby/buildkit/issues/1189
				bklog.G(ctx).Debugf("failed to set xattr %s of path %s during apply", xattr, ca.dstPath)
			}
		}
	}

	if ca.setOpaque {
		// This is set in the case where we are creating a directory that is replacing a whiteout device
		xattr := opaqueXattr(a.userxattr)
		if err := sysx.LSetxattr(ca.dstPath, xattr, []byte{'y'}, 0); err != nil {
			return errors.Wrapf(err, "failed to set opaque xattr %q of path %s", xattr, ca.dstPath)
		}
	}

	atimeSpec := unix.Timespec{Sec: ca.srcStat.Atim.Sec, Nsec: ca.srcStat.Atim.Nsec}
	mtimeSpec := unix.Timespec{Sec: ca.srcStat.Mtim.Sec, Nsec: ca.srcStat.Mtim.Nsec}
	if ca.srcStat.Mode&unix.S_IFMT != unix.S_IFDIR {
		// apply times immediately for non-dirs
		if err := unix.UtimesNanoAt(unix.AT_FDCWD, ca.dstPath, []unix.Timespec{atimeSpec, mtimeSpec}, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return err
		}
	} else {
		// save the times we should set on this dir, to be applied after subfiles have been set
		a.dirModTimes[ca.dstPath] = mtimeSpec
	}

	return nil
}

func (a *applier) Flush() error {
	// Set dir times now that everything has been modified. Walk the filesystem tree to ensure
	// that we never try to apply to a path that has been deleted or modified since times for it
	// were stored. This is needed for corner cases such as where a parent dir is removed and
	// replaced with a symlink.
	return filepath.WalkDir(a.root, func(path string, d gofs.DirEntry, prevErr error) error {
		if prevErr != nil {
			return prevErr
		}
		if !d.IsDir() {
			return nil
		}
		if mtime, ok := a.dirModTimes[path]; ok {
			if err := unix.UtimesNanoAt(unix.AT_FDCWD, path, []unix.Timespec{{Nsec: unix.UTIME_OMIT}, mtime}, unix.AT_SYMLINK_NOFOLLOW); err != nil {
				return err
			}
		}
		return nil
	})
}

func (a *applier) Release() error {
	if a.release != nil {
		err := a.release()
		if err != nil {
			return err
		}
	}
	a.release = nil
	return nil
}

func (a *applier) Usage() (snapshots.Usage, error) {
	// Calculate the disk space used under the apply root, similar to the normal containerd snapshotter disk usage
	// calculations but with the extra ability to take into account hardlinks that were created between snapshots, ensuring that
	// they don't get double counted.
	inodes := make(map[inode]struct{})
	var usage snapshots.Usage
	if err := filepath.WalkDir(a.root, func(path string, dirent gofs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := dirent.Info()
		if err != nil {
			return err
		}
		stat := info.Sys().(*syscall.Stat_t)
		inode := statInode(stat)
		if _, ok := inodes[inode]; ok {
			return nil
		}
		inodes[inode] = struct{}{}
		if a.crossSnapshotLinks != nil {
			if _, ok := a.crossSnapshotLinks[statInode(stat)]; ok {
				// don't count cross-snapshot hardlinks
				return nil
			}
		}
		usage.Inodes++
		usage.Size += stat.Blocks * 512 // 512 is always block size, see "man 2 stat"
		return nil
	}); err != nil {
		return snapshots.Usage{}, err
	}
	return usage, nil
}

type differ struct {
	lowerRoot    string
	releaseLower func() error

	upperRoot    string
	releaseUpper func() error

	upperBindSource  string
	upperOverlayDirs []string // ordered lowest -> highest

	upperdir string

	visited map[string]struct{} // set of parent subPaths that have been visited
	inodes  map[inode]string    // map of inode -> subPath
}

func differFor(lowerMntable, upperMntable Mountable) (_ *differ, rerr error) {
	d := &differ{
		visited: make(map[string]struct{}),
		inodes:  make(map[inode]string),
	}
	defer func() {
		if rerr != nil {
			rerr = multierror.Append(rerr, d.Release()).ErrorOrNil()
		}
	}()

	var lowerMnts []mount.Mount
	if lowerMntable != nil {
		mnts, release, err := lowerMntable.Mount()
		if err != nil {
			return nil, err
		}
		mounter := LocalMounterWithMounts(mnts)
		root, err := mounter.Mount()
		if err != nil {
			return nil, err
		}
		d.lowerRoot = root
		lowerMnts = mnts
		d.releaseLower = func() error {
			err := mounter.Unmount()
			return multierror.Append(err, release()).ErrorOrNil()
		}
	}

	var upperMnts []mount.Mount
	if upperMntable != nil {
		mnts, release, err := upperMntable.Mount()
		if err != nil {
			return nil, err
		}
		mounter := LocalMounterWithMounts(mnts)
		root, err := mounter.Mount()
		if err != nil {
			return nil, err
		}
		d.upperRoot = root
		upperMnts = mnts
		d.releaseUpper = func() error {
			err := mounter.Unmount()
			return multierror.Append(err, release()).ErrorOrNil()
		}
	}

	if len(upperMnts) == 1 {
		switch upperMnts[0].Type {
		case "bind", "rbind":
			d.upperBindSource = upperMnts[0].Source
		case "overlay":
			overlayDirs, err := overlay.GetOverlayLayers(upperMnts[0])
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get overlay layers from mount %+v", upperMnts[0])
			}
			d.upperOverlayDirs = overlayDirs
		}
	}
	if len(lowerMnts) > 0 {
		if upperdir, err := overlay.GetUpperdir(lowerMnts, upperMnts); err == nil {
			d.upperdir = upperdir
		}
	}

	return d, nil
}

func (d *differ) HandleChanges(ctx context.Context, handle func(context.Context, *change) error) error {
	if d.upperdir != "" {
		return d.overlayChanges(ctx, handle)
	}
	return d.doubleWalkingChanges(ctx, handle)
}

func (d *differ) doubleWalkingChanges(ctx context.Context, handle func(context.Context, *change) error) error {
	return fs.Changes(ctx, d.lowerRoot, d.upperRoot, func(kind fs.ChangeKind, subPath string, srcfi os.FileInfo, prevErr error) error {
		if prevErr != nil {
			return prevErr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if kind == fs.ChangeKindUnmodified {
			return nil
		}

		// NOTE: it's tempting to skip creating parent dirs when change kind is Delete, but
		// that would make us incompatible with the image exporter code:
		// https://github.com/containerd/containerd/pull/2095
		if err := d.checkParent(ctx, subPath, handle); err != nil {
			return errors.Wrapf(err, "failed to check parent for %s", subPath)
		}

		c := &change{
			kind:    kind,
			subPath: subPath,
		}

		if srcfi != nil {
			// Try to ensure that srcPath and srcStat are set to a file from the underlying filesystem
			// rather than the actual mount when possible. This allows hardlinking without getting EXDEV.
			switch {
			case !srcfi.IsDir() && d.upperBindSource != "":
				srcPath, err := safeJoin(d.upperBindSource, c.subPath)
				if err != nil {
					return errors.Wrapf(err, "failed to join %s and %s", d.upperBindSource, c.subPath)
				}
				c.srcPath = srcPath
				if fi, err := os.Lstat(c.srcPath); err == nil {
					srcfi = fi
				} else {
					return errors.Wrap(err, "failed to stat underlying file from bind mount")
				}
			case !srcfi.IsDir() && len(d.upperOverlayDirs) > 0:
				for i := range d.upperOverlayDirs {
					dir := d.upperOverlayDirs[len(d.upperOverlayDirs)-1-i]
					path, err := safeJoin(dir, c.subPath)
					if err != nil {
						return errors.Wrapf(err, "failed to join %s and %s", dir, c.subPath)
					}
					if stat, err := os.Lstat(path); err == nil {
						c.srcPath = path
						srcfi = stat
						break
					} else if errors.Is(err, unix.ENOENT) {
						continue
					} else {
						return errors.Wrap(err, "failed to lstat when finding direct path of overlay file")
					}
				}
			default:
				srcPath, err := safeJoin(d.upperRoot, subPath)
				if err != nil {
					return errors.Wrapf(err, "failed to join %s and %s", d.upperRoot, subPath)
				}
				c.srcPath = srcPath
				if fi, err := os.Lstat(c.srcPath); err == nil {
					srcfi = fi
				} else {
					return errors.Wrap(err, "failed to stat srcPath from differ")
				}
			}

			var ok bool
			c.srcStat, ok = srcfi.Sys().(*syscall.Stat_t)
			if !ok {
				return errors.Errorf("unhandled stat type for %+v", srcfi)
			}

			if !srcfi.IsDir() && c.srcStat.Nlink > 1 {
				if linkSubPath, ok := d.inodes[statInode(c.srcStat)]; ok {
					c.linkSubPath = linkSubPath
				} else {
					d.inodes[statInode(c.srcStat)] = c.subPath
				}
			}
		}

		return handle(ctx, c)
	})
}

func (d *differ) overlayChanges(ctx context.Context, handle func(context.Context, *change) error) error {
	return overlay.Changes(ctx, func(kind fs.ChangeKind, subPath string, srcfi os.FileInfo, prevErr error) error {
		if prevErr != nil {
			return prevErr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if kind == fs.ChangeKindUnmodified {
			return nil
		}

		if err := d.checkParent(ctx, subPath, handle); err != nil {
			return errors.Wrapf(err, "failed to check parent for %s", subPath)
		}

		srcPath, err := safeJoin(d.upperdir, subPath)
		if err != nil {
			return errors.Wrapf(err, "failed to join %s and %s", d.upperdir, subPath)
		}

		c := &change{
			kind:    kind,
			subPath: subPath,
			srcPath: srcPath,
		}

		if srcfi != nil {
			var ok bool
			c.srcStat, ok = srcfi.Sys().(*syscall.Stat_t)
			if !ok {
				return errors.Errorf("unhandled stat type for %+v", srcfi)
			}

			if !srcfi.IsDir() && c.srcStat.Nlink > 1 {
				if linkSubPath, ok := d.inodes[statInode(c.srcStat)]; ok {
					c.linkSubPath = linkSubPath
				} else {
					d.inodes[statInode(c.srcStat)] = c.subPath
				}
			}
		}

		return handle(ctx, c)
	}, d.upperdir, d.upperRoot, d.lowerRoot)
}

func (d *differ) checkParent(ctx context.Context, subPath string, handle func(context.Context, *change) error) error {
	parentSubPath := filepath.Dir(subPath)
	if parentSubPath == "/" {
		return nil
	}
	if _, ok := d.visited[parentSubPath]; ok {
		return nil
	}
	d.visited[parentSubPath] = struct{}{}

	if err := d.checkParent(ctx, parentSubPath, handle); err != nil {
		return err
	}
	parentSrcPath, err := safeJoin(d.upperRoot, parentSubPath)
	if err != nil {
		return err
	}
	srcfi, err := os.Lstat(parentSrcPath)
	if err != nil {
		return err
	}
	parentSrcStat, ok := srcfi.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.Errorf("unexpected type %T", srcfi)
	}
	return handle(ctx, &change{
		kind:    fs.ChangeKindModify,
		subPath: parentSubPath,
		srcPath: parentSrcPath,
		srcStat: parentSrcStat,
	})
}

func (d *differ) Release() error {
	var err error
	if d.releaseLower != nil {
		err = d.releaseLower()
		if err == nil {
			d.releaseLower = nil
		}
	}
	if d.releaseUpper != nil {
		err = multierror.Append(err, d.releaseUpper()).ErrorOrNil()
		if err == nil {
			d.releaseUpper = nil
		}
	}
	return err
}

func safeJoin(root, path string) (string, error) {
	dir, base := filepath.Split(path)
	parent, err := fs.RootPath(root, dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, base), nil
}

const (
	trustedOpaqueXattr = "trusted.overlay.opaque"
	userOpaqueXattr    = "user.overlay.opaque"
)

func isOpaqueXattr(s string) bool {
	for _, k := range []string{trustedOpaqueXattr, userOpaqueXattr} {
		if s == k {
			return true
		}
	}
	return false
}

func opaqueXattr(userxattr bool) string {
	if userxattr {
		return userOpaqueXattr
	}
	return trustedOpaqueXattr
}

// needsUserXAttr checks whether overlay mounts should be provided the userxattr option. We can't use
// NeedsUserXAttr from the overlayutils package directly because we don't always have direct knowledge
// of the root of the snapshotter state (such as when using a remote snapshotter). Instead, we create
// a temporary new snapshot and test using its root, which works because single layer snapshots will
// use bind-mounts even when created by an overlay based snapshotter.
func needsUserXAttr(ctx context.Context, sn Snapshotter, lm leases.Manager) (bool, error) {
	key := identity.NewID()

	ctx, done, err := leaseutil.WithLease(ctx, lm, leaseutil.MakeTemporary)
	if err != nil {
		return false, errors.Wrap(err, "failed to create lease for checking user xattr")
	}
	defer done(context.TODO())

	err = sn.Prepare(ctx, key, "")
	if err != nil {
		return false, err
	}
	mntable, err := sn.Mounts(ctx, key)
	if err != nil {
		return false, err
	}
	mnts, unmount, err := mntable.Mount()
	if err != nil {
		return false, err
	}
	defer unmount()

	var userxattr bool
	if err := mount.WithTempMount(ctx, mnts, func(root string) error {
		var err error
		userxattr, err = overlayutils.NeedsUserXAttr(root)
		return err
	}); err != nil {
		return false, err
	}
	return userxattr, nil
}
