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

package mount

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/containerd/containerd/sys"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

var (
	pagesize              = 4096
	allowedHelperBinaries = []string{"mount.fuse", "mount.fuse3"}
)

func init() {
	pagesize = os.Getpagesize()
}

// Mount to the provided target path.
//
// If m.Type starts with "fuse." or "fuse3.", "mount.fuse" or "mount.fuse3"
// helper binary is called.
func (m *Mount) Mount(target string) error {
	for _, helperBinary := range allowedHelperBinaries {
		// helperBinary = "mount.fuse", typePrefix = "fuse."
		typePrefix := strings.TrimPrefix(helperBinary, "mount.") + "."
		if strings.HasPrefix(m.Type, typePrefix) {
			return m.mountWithHelper(helperBinary, typePrefix, target)
		}
	}
	var (
		chdir   string
		options = m.Options
	)

	// avoid hitting one page limit of mount argument buffer
	//
	// NOTE: 512 is a buffer during pagesize check.
	if m.Type == "overlay" && optionsSize(options) >= pagesize-512 {
		chdir, options = compactLowerdirOption(options)
	}

	flags, data := parseMountOptions(options)
	if len(data) > pagesize {
		return errors.Errorf("mount options is too long")
	}

	// propagation types.
	const ptypes = unix.MS_SHARED | unix.MS_PRIVATE | unix.MS_SLAVE | unix.MS_UNBINDABLE

	// Ensure propagation type change flags aren't included in other calls.
	oflags := flags &^ ptypes

	// In the case of remounting with changed data (data != ""), need to call mount (moby/moby#34077).
	if flags&unix.MS_REMOUNT == 0 || data != "" {
		// Initial call applying all non-propagation flags for mount
		// or remount with changed data
		if err := mountAt(chdir, m.Source, target, m.Type, uintptr(oflags), data); err != nil {
			return err
		}
	}

	if flags&ptypes != 0 {
		// Change the propagation type.
		const pflags = ptypes | unix.MS_REC | unix.MS_SILENT
		if err := unix.Mount("", target, "", uintptr(flags&pflags), ""); err != nil {
			return err
		}
	}

	const broflags = unix.MS_BIND | unix.MS_RDONLY
	if oflags&broflags == broflags {
		// Remount the bind to apply read only.
		return unix.Mount("", target, "", uintptr(oflags|unix.MS_REMOUNT), "")
	}
	return nil
}

// Unmount the provided mount path with the flags
func Unmount(target string, flags int) error {
	if err := unmount(target, flags); err != nil && err != unix.EINVAL {
		return err
	}
	return nil
}

func isFUSE(dir string) (bool, error) {
	// fuseSuperMagic is defined in statfs(2)
	const fuseSuperMagic = 0x65735546
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return false, err
	}
	return st.Type == fuseSuperMagic, nil
}

func unmount(target string, flags int) error {
	// For FUSE mounts, attempting to execute fusermount helper binary is preferred
	// https://github.com/containerd/containerd/pull/3765#discussion_r342083514
	if ok, err := isFUSE(target); err == nil && ok {
		for _, helperBinary := range []string{"fusermount3", "fusermount"} {
			cmd := exec.Command(helperBinary, "-u", target)
			if err := cmd.Run(); err == nil {
				return nil
			}
			// ignore error and try unix.Unmount
		}
	}
	for i := 0; i < 50; i++ {
		if err := unix.Unmount(target, flags); err != nil {
			switch err {
			case unix.EBUSY:
				time.Sleep(50 * time.Millisecond)
				continue
			default:
				return err
			}
		}
		return nil
	}
	return errors.Wrapf(unix.EBUSY, "failed to unmount target %s", target)
}

// UnmountAll repeatedly unmounts the given mount point until there
// are no mounts remaining (EINVAL is returned by mount), which is
// useful for undoing a stack of mounts on the same mount point.
// UnmountAll all is noop when the first argument is an empty string.
// This is done when the containerd client did not specify any rootfs
// mounts (e.g. because the rootfs is managed outside containerd)
// UnmountAll is noop when the mount path does not exist.
func UnmountAll(mount string, flags int) error {
	if mount == "" {
		return nil
	}
	if _, err := os.Stat(mount); os.IsNotExist(err) {
		return nil
	}

	for {
		if err := unmount(mount, flags); err != nil {
			// EINVAL is returned if the target is not a
			// mount point, indicating that we are
			// done. It can also indicate a few other
			// things (such as invalid flags) which we
			// unfortunately end up squelching here too.
			if err == unix.EINVAL {
				return nil
			}
			return err
		}
	}
}

// parseMountOptions takes fstab style mount options and parses them for
// use with a standard mount() syscall
func parseMountOptions(options []string) (int, string) {
	var (
		flag int
		data []string
	)
	flags := map[string]struct {
		clear bool
		flag  int
	}{
		"async":         {true, unix.MS_SYNCHRONOUS},
		"atime":         {true, unix.MS_NOATIME},
		"bind":          {false, unix.MS_BIND},
		"defaults":      {false, 0},
		"dev":           {true, unix.MS_NODEV},
		"diratime":      {true, unix.MS_NODIRATIME},
		"dirsync":       {false, unix.MS_DIRSYNC},
		"exec":          {true, unix.MS_NOEXEC},
		"mand":          {false, unix.MS_MANDLOCK},
		"noatime":       {false, unix.MS_NOATIME},
		"nodev":         {false, unix.MS_NODEV},
		"nodiratime":    {false, unix.MS_NODIRATIME},
		"noexec":        {false, unix.MS_NOEXEC},
		"nomand":        {true, unix.MS_MANDLOCK},
		"norelatime":    {true, unix.MS_RELATIME},
		"nostrictatime": {true, unix.MS_STRICTATIME},
		"nosuid":        {false, unix.MS_NOSUID},
		"rbind":         {false, unix.MS_BIND | unix.MS_REC},
		"relatime":      {false, unix.MS_RELATIME},
		"remount":       {false, unix.MS_REMOUNT},
		"ro":            {false, unix.MS_RDONLY},
		"rw":            {true, unix.MS_RDONLY},
		"strictatime":   {false, unix.MS_STRICTATIME},
		"suid":          {true, unix.MS_NOSUID},
		"sync":          {false, unix.MS_SYNCHRONOUS},
	}
	for _, o := range options {
		// If the option does not exist in the flags table or the flag
		// is not supported on the platform,
		// then it is a data value for a specific fs type
		if f, exists := flags[o]; exists && f.flag != 0 {
			if f.clear {
				flag &^= f.flag
			} else {
				flag |= f.flag
			}
		} else {
			data = append(data, o)
		}
	}
	return flag, strings.Join(data, ",")
}

// compactLowerdirOption updates overlay lowdir option and returns the common
// dir among all the lowdirs.
func compactLowerdirOption(opts []string) (string, []string) {
	idx, dirs := findOverlayLowerdirs(opts)
	if idx == -1 || len(dirs) == 1 {
		// no need to compact if there is only one lowerdir
		return "", opts
	}

	// find out common dir
	commondir := longestCommonPrefix(dirs)
	if commondir == "" {
		return "", opts
	}

	// NOTE: the snapshot id is based on digits.
	// in order to avoid to get snapshots/x, should be back to parent dir.
	// however, there is assumption that the common dir is ${root}/io.containerd.v1.overlayfs/snapshots.
	commondir = path.Dir(commondir)
	if commondir == "/" {
		return "", opts
	}
	commondir = commondir + "/"

	newdirs := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		newdirs = append(newdirs, dir[len(commondir):])
	}

	newopts := copyOptions(opts)
	newopts = append(newopts[:idx], newopts[idx+1:]...)
	newopts = append(newopts, fmt.Sprintf("lowerdir=%s", strings.Join(newdirs, ":")))
	return commondir, newopts
}

// findOverlayLowerdirs returns the index of lowerdir in mount's options and
// all the lowerdir target.
func findOverlayLowerdirs(opts []string) (int, []string) {
	var (
		idx    = -1
		prefix = "lowerdir="
	)

	for i, opt := range opts {
		if strings.HasPrefix(opt, prefix) {
			idx = i
			break
		}
	}

	if idx == -1 {
		return -1, nil
	}
	return idx, strings.Split(opts[idx][len(prefix):], ":")
}

// longestCommonPrefix finds the longest common prefix in the string slice.
func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	} else if len(strs) == 1 {
		return strs[0]
	}

	// find out the min/max value by alphabetical order
	min, max := strs[0], strs[0]
	for _, str := range strs[1:] {
		if min > str {
			min = str
		}
		if max < str {
			max = str
		}
	}

	// find out the common part between min and max
	for i := 0; i < len(min) && i < len(max); i++ {
		if min[i] != max[i] {
			return min[:i]
		}
	}
	return min
}

// copyOptions copies the options.
func copyOptions(opts []string) []string {
	if len(opts) == 0 {
		return nil
	}

	acopy := make([]string, len(opts))
	copy(acopy, opts)
	return acopy
}

// optionsSize returns the byte size of options of mount.
func optionsSize(opts []string) int {
	size := 0
	for _, opt := range opts {
		size += len(opt)
	}
	return size
}

func mountAt(chdir string, source, target, fstype string, flags uintptr, data string) error {
	if chdir == "" {
		return unix.Mount(source, target, fstype, flags, data)
	}

	f, err := os.Open(chdir)
	if err != nil {
		return errors.Wrap(err, "failed to mountat")
	}
	defer f.Close()

	fs, err := f.Stat()
	if err != nil {
		return errors.Wrap(err, "failed to mountat")
	}

	if !fs.IsDir() {
		return errors.Wrap(errors.Errorf("%s is not dir", chdir), "failed to mountat")
	}
	return errors.Wrap(sys.FMountat(f.Fd(), source, target, fstype, flags, data), "failed to mountat")
}

func (m *Mount) mountWithHelper(helperBinary, typePrefix, target string) error {
	// helperBinary: "mount.fuse3"
	// target: "/foo/merged"
	// m.Type: "fuse3.fuse-overlayfs"
	// command: "mount.fuse3 overlay /foo/merged -o lowerdir=/foo/lower2:/foo/lower1,upperdir=/foo/upper,workdir=/foo/work -t fuse-overlayfs"
	args := []string{m.Source, target}
	for _, o := range m.Options {
		args = append(args, "-o", o)
	}
	args = append(args, "-t", strings.TrimPrefix(m.Type, typePrefix))
	cmd := exec.Command(helperBinary, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "mount helper [%s %v] failed: %q", helperBinary, args, string(out))
	}
	return nil
}
