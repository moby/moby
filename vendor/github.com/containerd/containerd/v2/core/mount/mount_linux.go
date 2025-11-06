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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containerd/log"
	"github.com/moby/sys/userns"
	"golang.org/x/sys/unix"
)

type mountOpt struct {
	flags   int
	data    []string
	losetup bool
	uidmap  string
	gidmap  string
}

var (
	pagesize              = 4096
	allowedHelperBinaries = []string{"mount.fuse", "mount.fuse3"}
)

func init() {
	pagesize = os.Getpagesize()
}

// prepareIDMappedOverlay is a helper function to obtain
// actual "lowerdir=..." mount options. It creates and
// applies id mapping for each lowerdir.
//
// It returns:
//  1. New options that include new "lowedir=..." mount option.
//  2. "Clean up" function -- it should be called only if no error is returned.
//  3. Error -- nil if everything's fine, otherwise an error.
func prepareIDMappedOverlay(usernsFd int, options []string) ([]string, func(), error) {
	lowerIdx, lowerDirs := findOverlayLowerdirs(options)
	if lowerIdx == -1 {
		return options, nil, fmt.Errorf("failed to parse overlay lowerdir's from given options")
	}

	tmpLowerdirs, idMapCleanUp, err := doPrepareIDMappedOverlay(tempMountLocation, lowerDirs, usernsFd)
	if err != nil {
		return options, idMapCleanUp, fmt.Errorf("failed to create idmapped mount: %w", err)
	}

	options = append(options[:lowerIdx], options[lowerIdx+1:]...)
	options = append(options, fmt.Sprintf("lowerdir=%s", strings.Join(tmpLowerdirs, ":")))

	return options, idMapCleanUp, nil
}

// Mount to the provided target path.
//
// If m.Type starts with "fuse." or "fuse3.", "mount.fuse" or "mount.fuse3"
// helper binary is called.
func (m *Mount) mount(target string) (err error) {
	for _, helperBinary := range allowedHelperBinaries {
		// helperBinary = "mount.fuse", typePrefix = "fuse."
		typePrefix := strings.TrimPrefix(helperBinary, "mount.") + "."
		if strings.HasPrefix(m.Type, typePrefix) {
			return m.mountWithHelper(helperBinary, typePrefix, target)
		}
	}
	var (
		chdir     string
		recalcOpt bool
		usernsFd  *os.File
		options   = m.Options
	)

	opt := parseMountOptions(options)
	// The only remapping of both GID and UID is supported
	if opt.uidmap != "" && opt.gidmap != "" {
		if usernsFd, err = GetUsernsFD(opt.uidmap, opt.gidmap); err != nil {
			return err
		}
		defer usernsFd.Close()

		// overlay expects lowerdir's to be remapped instead
		if m.Type == "overlay" {
			var (
				userNsCleanUp func()
			)
			options, userNsCleanUp, err = prepareIDMappedOverlay(int(usernsFd.Fd()), options)
			if err != nil {
				return fmt.Errorf("failed to prepare idmapped overlay: %w", err)
			}
			defer userNsCleanUp()

			// To not meet concurrency issues while using the same lowedirs
			// for different containers, replace them by temporary directories,
			if optionsSize(options) >= pagesize-512 {
				recalcOpt = true
			} else {
				opt = parseMountOptions(options)
			}
		}
	}

	// avoid hitting one page limit of mount argument buffer
	//
	// NOTE: 512 is a buffer during pagesize check.
	if m.Type == "overlay" && optionsSize(options) >= pagesize-512 {
		chdir, options = compactLowerdirOption(options)
		// recalculate opt in case of lowerdirs have been replaced
		// by idmapped ones OR idmapped mounts' not used/supported.
		if recalcOpt || (opt.uidmap == "" || opt.gidmap == "") {
			opt = parseMountOptions(options)
		}
	}

	// propagation types.
	const ptypes = unix.MS_SHARED | unix.MS_PRIVATE | unix.MS_SLAVE | unix.MS_UNBINDABLE

	// Ensure propagation type change flags aren't included in other calls.
	oflags := opt.flags &^ ptypes

	var loopParams LoopParams
	if opt.losetup {
		loopParams = LoopParams{
			Readonly:  oflags&unix.MS_RDONLY == unix.MS_RDONLY,
			Autoclear: true,
		}
		loopParams.Direct, opt.data = hasDirectIO(opt.data)
	}

	dataInStr := strings.Join(opt.data, ",")
	if len(dataInStr) > pagesize {
		return errors.New("mount options is too long")
	}

	// In the case of remounting with changed data (dataInStr != ""), need to call mount (moby/moby#34077).
	if opt.flags&unix.MS_REMOUNT == 0 || dataInStr != "" {
		// Initial call applying all non-propagation flags for mount
		// or remount with changed data
		source := m.Source
		if opt.losetup {
			loFile, err := setupLoop(m.Source, loopParams)
			if err != nil {
				return err
			}
			defer loFile.Close()

			// Mount the loop device instead
			source = loFile.Name()
		}
		if err := mountAt(chdir, source, target, m.Type, uintptr(oflags), dataInStr); err != nil {
			return err
		}
	}

	if opt.flags&ptypes != 0 {
		// Change the propagation type.
		const pflags = ptypes | unix.MS_REC | unix.MS_SILENT
		if err := unix.Mount("", target, "", uintptr(opt.flags&pflags), ""); err != nil {
			return err
		}
	}

	const broflags = unix.MS_BIND | unix.MS_RDONLY
	if oflags&broflags == broflags {
		// Preserve CL_UNPRIVILEGED "locked" flags of the
		// bind mount target when we remount to make the bind readonly.
		// This is necessary to ensure that
		// bind-mounting "with options" will not fail with user namespaces, due to
		// kernel restrictions that require user namespace mounts to preserve
		// CL_UNPRIVILEGED locked flags.
		var unprivFlags int
		if userns.RunningInUserNS() {
			unprivFlags, err = getUnprivilegedMountFlags(target)
			if err != nil {
				return err
			}
		}
		// Remount the bind to apply read only.
		return unix.Mount("", target, "", uintptr(oflags|unprivFlags|unix.MS_REMOUNT), "")
	}

	// remap non-overlay mount point
	if opt.uidmap != "" && opt.gidmap != "" && m.Type != "overlay" {
		if err := IDMapMount(target, target, int(usernsFd.Fd())); err != nil {
			return err
		}
	}
	return nil
}

// Get the set of mount flags that are set on the mount that contains the given
// path and are locked by CL_UNPRIVILEGED.
//
// From https://github.com/moby/moby/blob/v23.0.1/daemon/oci_linux.go#L430-L460
func getUnprivilegedMountFlags(path string) (int, error) {
	var statfs unix.Statfs_t
	if err := unix.Statfs(path, &statfs); err != nil {
		return 0, err
	}

	// The set of keys come from https://github.com/torvalds/linux/blob/v4.13/fs/namespace.c#L1034-L1048.
	unprivilegedFlags := []int{
		unix.MS_RDONLY,
		unix.MS_NODEV,
		unix.MS_NOEXEC,
		unix.MS_NOSUID,
		unix.MS_NOATIME,
		unix.MS_RELATIME,
		unix.MS_NODIRATIME,
	}

	var flags int
	for flag := range unprivilegedFlags {
		if int(statfs.Flags)&flag == flag {
			flags |= flag
		}
	}

	return flags, nil
}

func doPrepareIDMappedOverlay(tmpDir string, lowerDirs []string, usernsFd int) (_ []string, _ func(), retErr error) {
	commonDir, err := getCommonDirectory(lowerDirs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to determine common parent: %w", err)
	}

	tempRemountsLocation, err := os.MkdirTemp(tmpDir, "ovl-idmapped")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temporary overlay lowerdir mount location: %w", err)
	}
	cleanDir := func() {
		if err := os.Remove(tempRemountsLocation); err != nil {
			log.L.WithError(err).Infof("failed to remove idmapped directory")
		}
	}
	defer func() {
		if retErr != nil {
			cleanDir()
		}
	}()

	// IDMapMount the directory containing all the layers
	if err := IDMapMountWithAttrs(commonDir, tempRemountsLocation, usernsFd, unix.MOUNT_ATTR_RDONLY, 0); err != nil {
		return nil, nil, err
	}
	cleanMount := func() {
		// Use the Unmount helper that does retries because there can be easily an open fd
		// to the idmapped directory and when containerd forks to create a userns fd (maybe
		// for another container), it will make the mount busy for a few ms.
		err := Unmount(tempRemountsLocation, 0)
		if err != nil {
			log.L.WithError(err).Warnf("failed to unmount idmapped directory %s: %v", tempRemountsLocation, err)
		}
	}
	defer func() {
		if retErr != nil {
			cleanMount()
		}
	}()

	// Build new lower dir paths through the idmapped directory
	tmpLowerDirs := buildIDMappedPaths(lowerDirs, commonDir, tempRemountsLocation)

	cleanup := func() {
		cleanMount()
		cleanDir()
	}
	return tmpLowerDirs, cleanup, nil
}

// getCommonDirectory finds the common directory among the lowerDirs passed in.
// "/" and "." are considered invalid common directories and are treated as error
func getCommonDirectory(lowerDirs []string) (string, error) {
	commonPrefix := longestCommonPrefix(lowerDirs)
	if commonPrefix == "" {
		return "", fmt.Errorf("no common prefix found")
	}

	// Ensure the common prefix ends at a directory boundary
	commonPrefix = path.Dir(commonPrefix)

	if commonPrefix == "." || commonPrefix == "/" {
		return "", fmt.Errorf("invalid common directory: %s", commonPrefix)
	}

	return commonPrefix, nil
}

// buildIDMappedPaths constructs new lower directory paths through an idmapped mount of the commonDir.
// It takes the original lowerDirs, the commonDir of those dirs, and rewrites the paths
// to go through the idMappedDir directory to achieve idmapped lowerdirs ready for overlayfs
func buildIDMappedPaths(lowerDirs []string, commonDir, idMappedDir string) []string {
	tmpLowerDirs := make([]string, 0, len(lowerDirs))

	for _, lowerDir := range lowerDirs {
		relativePath := strings.TrimPrefix(lowerDir, commonDir)
		tmpLowerDirs = append(tmpLowerDirs, filepath.Join(idMappedDir, relativePath))
	}

	return tmpLowerDirs
}

// parseMountOptions takes fstab style mount options and parses them for
// use with a standard mount() syscall
func parseMountOptions(options []string) (opt mountOpt) {
	loopOpt := "loop"
	flagsMap := map[string]struct {
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
		if f, exists := flagsMap[o]; exists && f.flag != 0 {
			if f.clear {
				opt.flags &^= f.flag
			} else {
				opt.flags |= f.flag
			}
		} else if o == loopOpt {
			opt.losetup = true
		} else if strings.HasPrefix(o, "uidmap=") {
			opt.uidmap = strings.TrimPrefix(o, "uidmap=")
		} else if strings.HasPrefix(o, "gidmap=") {
			opt.gidmap = strings.TrimPrefix(o, "gidmap=")
		} else {
			opt.data = append(opt.data, o)
		}
	}
	return
}

func hasDirectIO(opts []string) (bool, []string) {
	for idx, opt := range opts {
		if opt == "direct-io" {
			return true, append(opts[:idx], opts[idx+1:]...)
		}
	}
	return false, opts
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
	if commondir == "/" || commondir == "." {
		return "", opts
	}
	commondir = commondir + "/"

	newdirs := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if len(dir) <= len(commondir) {
			return "", opts
		}
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
		err := unix.Mount(source, target, fstype, flags, data)
		if err != nil {
			return fmt.Errorf("mount source: %q, target: %q, fstype: %s, flags: %d, data: %q, err: %w", source, target, fstype, flags, data, err)
		}
		return nil
	}

	ch := make(chan error, 1)
	go func() {
		runtime.LockOSThread()

		// Do not unlock this thread.
		// If the thread is unlocked go will try to use it for other goroutines.
		// However it is not possible to restore the thread state after CLONE_FS.
		//
		// Once the goroutine exits the thread should eventually be terminated by go.

		if err := unix.Unshare(unix.CLONE_FS); err != nil {
			ch <- err
			return
		}

		if err := unix.Chdir(chdir); err != nil {
			ch <- err
			return
		}
		err := unix.Mount(source, target, fstype, flags, data)
		if err != nil {
			err = fmt.Errorf("mount source: %q, target: %q, fstype: %s, flags: %d, data: %q, err: %w", source, target, fstype, flags, data, err)
		}
		ch <- err
	}()
	return <-ch
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

	infoBeforeMount, err := Lookup(target)
	if err != nil {
		return err
	}

	// cmd.CombinedOutput() may intermittently return ECHILD because of our signal handling in shim.
	// See #4387 and wait(2).
	const retriesOnECHILD = 10
	for i := 0; i < retriesOnECHILD; i++ {
		cmd := exec.Command(helperBinary, args...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		if !errors.Is(err, unix.ECHILD) {
			return fmt.Errorf("mount helper [%s %v] failed: %q: %w", helperBinary, args, string(out), err)
		}
		// We got ECHILD, we are not sure whether the mount was successful.
		// If the mount ID has changed, we are sure we got some new mount, but still not sure it is fully completed.
		// So we attempt to unmount the new mount before retrying.
		infoAfterMount, err := Lookup(target)
		if err != nil {
			return err
		}
		if infoAfterMount.ID != infoBeforeMount.ID {
			_ = unmount(target, 0)
		}
	}
	return fmt.Errorf("mount helper [%s %v] failed with ECHILD (retried %d times)", helperBinary, args, retriesOnECHILD)
}
