package mount // import "github.com/docker/docker/pkg/mount"

import (
	"sort"
	"strconv"

	"github.com/sirupsen/logrus"
)

// mountError records an error from mount or unmount operation
type mountError struct {
	op             string
	source, target string
	flags          uintptr
	data           string
	err            error
}

func (e *mountError) Error() string {
	out := e.op + " "

	if e.source != "" {
		out += e.source + ":" + e.target
	} else {
		out += e.target
	}

	if e.flags != uintptr(0) {
		out += ", flags: 0x" + strconv.FormatUint(uint64(e.flags), 16)
	}
	if e.data != "" {
		out += ", data: " + e.data
	}

	out += ": " + e.err.Error()
	return out
}

// Cause returns the underlying cause of the error
func (e *mountError) Cause() error {
	return e.err
}

// GetMounts retrieves a list of mounts for the current running process,
// with an optional filter applied (use nil for no filter).
func GetMounts(f FilterFunc) ([]*Info, error) {
	return parseMountTable(f)
}

// Mounted determines if a specified mountpoint has been mounted.
// On Linux it looks at /proc/self/mountinfo.
func Mounted(mountpoint string) (bool, error) {
	entries, err := GetMounts(SingleEntryFilter(mountpoint))
	if err != nil {
		return false, err
	}

	return len(entries) > 0, nil
}

// Mount will mount filesystem according to the specified configuration.
// Options must be specified like the mount or fstab unix commands:
// "opt1=val1,opt2=val2". See flags.go for supported option flags.
func Mount(device, target, mType, options string) error {
	flag, data := parseOptions(options)
	return mount(device, target, mType, uintptr(flag), data)
}

// ForceMount will mount filesystem according to the specified configuration.
// Options must be specified like the mount or fstab unix commands:
// "opt1=val1,opt2=val2". See flags.go for supported option flags.
//
// Deprecated: use Mount instead.
func ForceMount(device, target, mType, options string) error {
	flag, data := parseOptions(options)
	return mount(device, target, mType, uintptr(flag), data)
}

// Unmount lazily unmounts a filesystem on supported platforms, otherwise
// does a normal unmount.
func Unmount(target string) error {
	return unmount(target, mntDetach)
}

// RecursiveUnmount unmounts the target and all mounts underneath, starting with
// the deepsest mount first.
func RecursiveUnmount(target string) error {
	mounts, err := parseMountTable(PrefixFilter(target))
	if err != nil {
		return err
	}

	// Make the deepest mount be first
	sort.Slice(mounts, func(i, j int) bool {
		return len(mounts[i].Mountpoint) > len(mounts[j].Mountpoint)
	})

	for i, m := range mounts {
		logrus.Debugf("Trying to unmount %s", m.Mountpoint)
		err = unmount(m.Mountpoint, mntDetach)
		if err != nil {
			if i == len(mounts)-1 { // last mount
				return err
			}
			// This is some submount, we can ignore this error for now, the final unmount will fail if this is a real problem
			logrus.WithError(err).Warnf("Failed to unmount submount %s", m.Mountpoint)
		}

		logrus.Debugf("Unmounted %s", m.Mountpoint)
	}
	return nil
}
