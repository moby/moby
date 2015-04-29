// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package user

import (
	"io"
	"os"
)

// Unix-specific path to the passwd and group formatted files.
const (
	unixPasswdFile = "/etc/passwd"
	unixGroupFile  = "/etc/group"
)

func GetPasswdFile() (string, error) {
	return unixPasswdFile, nil
}

func GetPasswd() (io.ReadCloser, error) {
	return os.Open(unixPasswdFile)
}

func GetGroupFile() (string, error) {
	return unixGroupFile, nil
}

func GetGroup() (io.ReadCloser, error) {
	return os.Open(unixGroupFile)
}
