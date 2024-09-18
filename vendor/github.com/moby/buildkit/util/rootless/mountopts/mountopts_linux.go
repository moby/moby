package mountopts

import (
	"github.com/containerd/containerd/mount"
	"github.com/moby/buildkit/util/strutil"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// UnprivilegedMountFlags gets the set of mount flags that are set on the mount that contains the given
// path and are locked by CL_UNPRIVILEGED. This is necessary to ensure that
// bind-mounting "with options" will not fail with user namespaces, due to
// kernel restrictions that require user namespace mounts to preserve
// CL_UNPRIVILEGED locked flags.
//
// From https://github.com/moby/moby/blob/v23.0.1/daemon/oci_linux.go#L430-L460
func UnprivilegedMountFlags(path string) ([]string, error) {
	var statfs unix.Statfs_t
	if err := unix.Statfs(path, &statfs); err != nil {
		return nil, err
	}

	// The set of keys come from https://github.com/torvalds/linux/blob/v4.13/fs/namespace.c#L1034-L1048.
	unprivilegedFlags := map[uint64]string{
		unix.MS_RDONLY:     "ro",
		unix.MS_NODEV:      "nodev",
		unix.MS_NOEXEC:     "noexec",
		unix.MS_NOSUID:     "nosuid",
		unix.MS_NOATIME:    "noatime",
		unix.MS_RELATIME:   "relatime",
		unix.MS_NODIRATIME: "nodiratime",
	}

	var flags []string
	for mask, flag := range unprivilegedFlags {
		if uint64(statfs.Flags)&mask == mask {
			flags = append(flags, flag)
		}
	}

	return flags, nil
}

// FixUp is for https://github.com/moby/buildkit/issues/3098
func FixUp(mounts []mount.Mount) ([]mount.Mount, error) {
	for i, m := range mounts {
		var isBind bool
		for _, o := range m.Options {
			switch o {
			case "bind", "rbind":
				isBind = true
			}
		}
		if !isBind {
			continue
		}
		unpriv, err := UnprivilegedMountFlags(m.Source)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get unprivileged mount flags for %+v", m)
		}
		m.Options = strutil.DedupeSlice(append(m.Options, unpriv...))
		mounts[i] = m
	}
	return mounts, nil
}

func FixUpOCI(mounts []specs.Mount) ([]specs.Mount, error) {
	for i, m := range mounts {
		var isBind bool
		for _, o := range m.Options {
			switch o {
			case "bind", "rbind":
				isBind = true
			}
		}
		if !isBind {
			continue
		}
		unpriv, err := UnprivilegedMountFlags(m.Source)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get unprivileged mount flags for %+v", m)
		}
		m.Options = strutil.DedupeSlice(append(m.Options, unpriv...))
		mounts[i] = m
	}
	return mounts, nil
}
