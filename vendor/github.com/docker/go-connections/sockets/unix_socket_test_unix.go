//go:build !windows

package sockets

import (
	"net"
	"os"
	"syscall"
	"testing"
)

func createTestUnixSocket(t *testing.T, path string) (listener net.Listener) {
	uid, gid := os.Getuid(), os.Getgid()
	perms := os.FileMode(0660)
	l, err := NewUnixSocketWithOpts(path, WithChown(uid, gid), WithChmod(perms))
	if err != nil {
		t.Fatal(err)
	}
	p, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Mode().Perm() != perms {
		t.Fatalf("unexpected file permissions: expected: %#o, got: %#o", perms, p.Mode().Perm())
	}
	if stat, ok := p.Sys().(*syscall.Stat_t); ok {
		if stat.Uid != uint32(uid) || stat.Gid != uint32(gid) {
			t.Fatalf("unexpected file ownership: expected: %d:%d, got: %d:%d", uid, gid, stat.Uid, stat.Gid)
		}
	}
	return l
}
