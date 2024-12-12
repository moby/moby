//go:build freebsd

package archive // import "github.com/docker/docker/pkg/archive"

import "golang.org/x/sys/unix"

var mknod = unix.Mknod
