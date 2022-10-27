//go:build linux || freebsd
// +build linux freebsd

package system // import "github.com/docker/docker/pkg/system"

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"gotest.tools/v3/assert"
)

// TestFromStatT tests fromStatT for a tempfile
func TestFromStatT(t *testing.T) {
	file := filepath.Join(t.TempDir(), "exist")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	stat := &syscall.Stat_t{}
	err := syscall.Lstat(file, stat)
	assert.NilError(t, err)

	s, err := fromStatT(stat)
	assert.NilError(t, err)

	if stat.Mode != s.Mode() {
		t.Fatal("got invalid mode")
	}
	if stat.Uid != s.UID() {
		t.Fatal("got invalid uid")
	}
	if stat.Gid != s.GID() {
		t.Fatal("got invalid gid")
	}
	//nolint:unconvert // conversion needed to fix mismatch types on mips64el
	if uint64(stat.Rdev) != s.Rdev() {
		t.Fatal("got invalid rdev")
	}
	if stat.Mtim != s.Mtim() {
		t.Fatal("got invalid mtim")
	}
}
