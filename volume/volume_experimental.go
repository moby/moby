// +build experimental

package volume

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/opencontainers/runc/libcontainer/user"
)

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
	excludePath := []string{"/", "/usr", "/etc"}
	for _, p := range excludePath {
		if fpath == p {
			return fmt.Errorf("Chowning of %s is not allowed", fpath)
		}
	}
	callback := func(p string, info os.FileInfo, err error) error {
		return os.Chown(p, uid, gid)
	}

	return filepath.Walk(fpath, callback)
}

// FixUIDGID recursively chown the content on container
// creation iff user specifies :u on volume mount and --user at the
// same time.
func FixUIDGID(userid, src, modes string) error {
	chownUser := false
	if userid == "" {
		return nil
	}

	arr := strings.Split(modes, ",")
	for _, mode := range arr {
		if mode == "u" {
			chownUser = true
			break
		}
	}
	if !chownUser {
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
