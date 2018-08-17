// +build linux darwin freebsd solaris

package driver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/containerd/continuity/devices"
	"github.com/containerd/continuity/sysx"
)

func (d *driver) Mknod(path string, mode os.FileMode, major, minor int) error {
	return devices.Mknod(path, mode, major, minor)
}

func (d *driver) Mkfifo(path string, mode os.FileMode) error {
	if mode&os.ModeNamedPipe == 0 {
		return errors.New("mode passed to Mkfifo does not have the named pipe bit set")
	}
	// mknod with a mode that has ModeNamedPipe set creates a fifo, not a
	// device.
	return devices.Mknod(path, mode, 0, 0)
}

// Lchmod changes the mode of an file not following symlinks.
func (d *driver) Lchmod(path string, mode os.FileMode) (err error) {
	if !filepath.IsAbs(path) {
		path, err = filepath.Abs(path)
		if err != nil {
			return
		}
	}

	return sysx.Fchmodat(0, path, uint32(mode), sysx.AtSymlinkNofollow)
}

// Getxattr returns all of the extended attributes for the file at path p.
func (d *driver) Getxattr(p string) (map[string][]byte, error) {
	xattrs, err := sysx.Listxattr(p)
	if err != nil {
		return nil, fmt.Errorf("listing %s xattrs: %v", p, err)
	}

	sort.Strings(xattrs)
	m := make(map[string][]byte, len(xattrs))

	for _, attr := range xattrs {
		value, err := sysx.Getxattr(p, attr)
		if err != nil {
			return nil, fmt.Errorf("getting %q xattr on %s: %v", attr, p, err)
		}

		// NOTE(stevvooe): This append/copy tricky relies on unique
		// xattrs. Break this out into an alloc/copy if xattrs are no
		// longer unique.
		m[attr] = append(m[attr], value...)
	}

	return m, nil
}

// Setxattr sets all of the extended attributes on file at path, following
// any symbolic links, if necessary. All attributes on the target are
// replaced by the values from attr. If the operation fails to set any
// attribute, those already applied will not be rolled back.
func (d *driver) Setxattr(path string, attrMap map[string][]byte) error {
	for attr, value := range attrMap {
		if err := sysx.Setxattr(path, attr, value, 0); err != nil {
			return fmt.Errorf("error setting xattr %q on %s: %v", attr, path, err)
		}
	}

	return nil
}

// LGetxattr returns all of the extended attributes for the file at path p
// not following symbolic links.
func (d *driver) LGetxattr(p string) (map[string][]byte, error) {
	xattrs, err := sysx.LListxattr(p)
	if err != nil {
		return nil, fmt.Errorf("listing %s xattrs: %v", p, err)
	}

	sort.Strings(xattrs)
	m := make(map[string][]byte, len(xattrs))

	for _, attr := range xattrs {
		value, err := sysx.LGetxattr(p, attr)
		if err != nil {
			return nil, fmt.Errorf("getting %q xattr on %s: %v", attr, p, err)
		}

		// NOTE(stevvooe): This append/copy tricky relies on unique
		// xattrs. Break this out into an alloc/copy if xattrs are no
		// longer unique.
		m[attr] = append(m[attr], value...)
	}

	return m, nil
}

// LSetxattr sets all of the extended attributes on file at path, not
// following any symbolic links. All attributes on the target are
// replaced by the values from attr. If the operation fails to set any
// attribute, those already applied will not be rolled back.
func (d *driver) LSetxattr(path string, attrMap map[string][]byte) error {
	for attr, value := range attrMap {
		if err := sysx.LSetxattr(path, attr, value, 0); err != nil {
			return fmt.Errorf("error setting xattr %q on %s: %v", attr, path, err)
		}
	}

	return nil
}

func (d *driver) DeviceInfo(fi os.FileInfo) (maj uint64, min uint64, err error) {
	return devices.DeviceInfo(fi)
}

// Readlink was forked on Windows to fix a Golang bug, use the "os" package here
func (d *driver) Readlink(p string) (string, error) {
	return os.Readlink(p)
}
