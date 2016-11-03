// +build linux

package mount

import (
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

// BindTree bind-mounts the tree recursively.
// See also docker/docker#20670.
func BindTree(source, dest string, rw bool) error {
	mounts, err := GetMounts()
	if err != nil {
		return err
	}
	opts := "bind,ro"
	if rw {
		opts = "bind,rw"
	}
	f := func(source, dest string) error {
		return Mount(source, dest, "bind", opts)
	}
	// the function is split for ease of mocking.
	return bindTree(mounts, source, dest, f)
}

func bindTree(mounts []*Info, source, dest string,
	f func(source, dest string) error) error {
	// sort the mounts in top-down order, using the number of '/' in m.Mountpoint
	sort.Sort(mountsByNumSep(mounts))
	hitSource := false
	for _, m := range mounts {
		if m.Mountpoint == source {
			hitSource = true
		}
		// typically m.Mountpoint="/dev/pts" (or just match the source),
		// source="/dev", dest="/var/lib/docker/overlay/foobar/merged/dev"
		if !strings.HasPrefix(m.Mountpoint, source) {
			continue
		}
		childSource := m.Mountpoint
		childDest := filepath.Join(filepath.Dir(dest), m.Mountpoint)
		if err := f(childSource, childDest); err != nil {
			return err
		}

	}
	if !hitSource {
		if err := f(source, dest); err != nil {
			return err
		}
	}
	return nil
}

// UnbindTree unmounts the bind-mounted tree recursively.
func UnbindTree(dest string) error {
	mounts, err := GetMounts()
	if err != nil {
		return err
	}
	f := func(dest string) error {
		// should we move MNT_DETACH to elsewhere?
		return syscall.Unmount(dest, syscall.MNT_DETACH)
	}
	// the function is split for ease of mocking,
	return unbindTree(mounts, dest, f)
}

func unbindTree(mounts []*Info, dest string,
	f func(dest string) error) error {
	// sort the mounts in bottom-up order, using the number of '/' in m.Mountpoint
	sort.Sort(sort.Reverse(mountsByNumSep(mounts)))
	hitDest := false
	for _, m := range mounts {
		if m.Mountpoint == dest {
			hitDest = true
		}
		// typically dest="/var/lib/docker/overlay/foobar/merged/dev"
		// m.Mountpoint="/var/lib/docker/overlay/foobar/merged/dev/pts"
		// (or just match the dest)
		if !strings.HasPrefix(m.Mountpoint, dest) {
			continue
		}

		if err := f(m.Mountpoint); err != nil {
			return err
		}
	}
	if !hitDest {
		if err := f(dest); err != nil {
			return err
		}
	}
	return nil
}

type mountsByNumSep []*Info

func (mounts mountsByNumSep) Len() int {
	return len(mounts)
}

func (mounts mountsByNumSep) Swap(i, j int) {
	mounts[i], mounts[j] = mounts[j], mounts[i]
}

func (mounts mountsByNumSep) Less(i, j int) bool {
	sep := "/"
	c := strings.Count(mounts[i].Mountpoint, sep)
	d := strings.Count(mounts[j].Mountpoint, sep)
	return c < d
}
