// +build !windows

package appdefaults

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	Address = "unix:///run/buildkit/buildkitd.sock"
	Root    = "/var/lib/buildkit"
)

// UserAddress typically returns /run/user/$UID/buildkit/buildkitd.sock
func UserAddress() string {
	//  pam_systemd sets XDG_RUNTIME_DIR but not other dirs.
	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntimeDir != "" {
		dirs := strings.Split(xdgRuntimeDir, ":")
		return "unix://" + filepath.Join(dirs[0], "buildkit", "buildkitd.sock")
	}
	return Address
}

// EnsureUserAddressDir sets sticky bit on XDG_RUNTIME_DIR if XDG_RUNTIME_DIR is set.
// See https://github.com/opencontainers/runc/issues/1694
func EnsureUserAddressDir() error {
	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntimeDir != "" {
		dirs := strings.Split(xdgRuntimeDir, ":")
		dir := filepath.Join(dirs[0], "buildkit")
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		return os.Chmod(dir, 0700|os.ModeSticky)
	}
	return nil
}

// UserRoot typically returns /home/$USER/.local/share/buildkit
func UserRoot() string {
	//  pam_systemd sets XDG_RUNTIME_DIR but not other dirs.
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome != "" {
		dirs := strings.Split(xdgDataHome, ":")
		return filepath.Join(dirs[0], "buildkit")
	}
	home := os.Getenv("HOME")
	if home != "" {
		return filepath.Join(home, ".local", "share", "buildkit")
	}
	return Root
}
