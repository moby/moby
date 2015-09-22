package volume

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/opencontainers/runc/libcontainer/user"
)

// DefaultDriverName is the driver name used for the driver
// implemented in the local package.
const DefaultDriverName string = "local"

// Driver is for creating and removing volumes.
type Driver interface {
	// Name returns the name of the volume driver.
	Name() string
	// Create makes a new volume with the given id.
	Create(name string, opts map[string]string) (Volume, error)
	// Remove deletes the volume.
	Remove(Volume) error
}

// Volume is a place to store data. It is backed by a specific driver, and can be mounted.
type Volume interface {
	// Name returns the name of the volume
	Name() string
	// DriverName returns the name of the driver which owns this volume.
	DriverName() string
	// Path returns the absolute path to the volume.
	Path() string
	// Mount mounts the volume and returns the absolute path to
	// where it can be consumed.
	Mount() (string, error)
	// Unmount unmounts the volume when it is no longer in use.
	Unmount() error
}

var validModeFlags = map[string]bool{
	"ro": true,
	"rw": true,
	"u":  true,
	"z":  true,
	"Z":  true,
}

func validMode(modes string) bool {
	arr := strings.Split(modes, ",")
	for _, mode := range arr {
		if !validModeFlags[mode] {
			return false
		}
	}
	return true
}

func rwMode(modes string) bool {
	arr := strings.Split(modes, ",")
	for _, mode := range arr {
		if mode == "ro" {
			return false
		}
	}
	return true
}

// ValidMountMode will make sure the mount mode is valid.
// returns if it's a valid mount mode or not.
func ValidMountMode(mode string) bool {
	return validMode(mode) || rwMode(mode)
}

// ReadWrite tells you if a mode string is a valid read-write mode or not.
func ReadWrite(mode string) bool {
	return rwMode(mode)
}

func rchown(fpath string, uid, gid int) error {
	exclude_path := []string{"/", "/usr", "/etc"}
	for _, p := range exclude_path {
		if fpath == p {
			return fmt.Errorf("Chowning of %s is not allowed", fpath)
		}
	}
	callback := func(p string, info os.FileInfo, err error) error {
		return os.Chown(p, uid, gid)
	}

	return filepath.Walk(fpath, callback)
}

func FixUidGid(userid, src, modes string) error {
	chown_user := false
	if userid == "" {
		return nil
	}

	arr := strings.Split(modes, ",")
	for _, mode := range arr {
		if mode == "u" {
			chown_user = true
			break
		}
	}
	if !chown_user {
		return nil
	}
	// Set up defaults.
	defaultExecUser := user.ExecUser{
		Uid:  syscall.Getuid(),
		Gid:  syscall.Getgid(),
		Home: "/",
	}
	passwdPath, err := user.GetPasswdPath()
	if err != nil {
		return err
	}
	groupPath, err := user.GetGroupPath()
	if err != nil {
		return err
	}
	execUser, err := user.GetExecUserPath(userid, &defaultExecUser, passwdPath, groupPath)
	if err != nil {
		return err
	}

	return rchown(src, execUser.Uid, execUser.Gid)
}
