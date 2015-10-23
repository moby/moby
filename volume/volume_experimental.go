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

var rwModes = map[string]bool{
	"rw":     true,
	"rw,Z":   true,
	"rw,z":   true,
	"z,rw":   true,
	"Z,rw":   true,
	"Z":      true,
	"z":      true,
	"u":      true,
	"rw,u":   true,
	"rw,Z,u": true,
	"rw,z,u": true,
	"z,rw,u": true,
	"Z,rw,u": true,
	"Z,u":    true,
	"z,u":    true,
}

// read-only modes
var roModes = map[string]bool{
	"ro":     true,
	"ro,Z":   true,
	"ro,z":   true,
	"z,ro":   true,
	"Z,ro":   true,
	"ro,u":   true,
	"ro,Z,u": true,
	"ro,z,u": true,
	"z,ro,u": true,
	"Z,ro,u": true,
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
	var chownUser bool
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
