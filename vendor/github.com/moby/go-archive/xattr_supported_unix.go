//go:build darwin || freebsd || netbsd

package archive

import "golang.org/x/sys/unix"

var noattr = unix.ENOATTR
