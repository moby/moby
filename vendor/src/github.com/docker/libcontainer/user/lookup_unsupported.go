// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!solaris

package user

import "io"

func GetPasswdFile() (string, error) {
	return "", ErrUnsupported
}

func GetPasswd() (io.ReadCloser, error) {
	return nil, ErrUnsupported
}

func GetGroupFile() (string, error) {
	return "", ErrUnsupported
}

func GetGroup() (io.ReadCloser, error) {
	return nil, ErrUnsupported
}
