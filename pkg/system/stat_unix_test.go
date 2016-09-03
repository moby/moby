// +build linux freebsd

package system

import (
	"os"
	"syscall"

	"github.com/go-check/check"
)

// TestFromStatT tests fromStatT for a tempfile
func (s *DockerSuite) TestFromStatT(c *check.C) {
	file, _, _, dir := prepareFiles(c)
	defer os.RemoveAll(dir)

	stat := &syscall.Stat_t{}
	err := syscall.Lstat(file, stat)

	st, err := fromStatT(stat)
	if err != nil {
		c.Fatal(err)
	}

	if stat.Mode != st.Mode() {
		c.Fatal("got invalid mode")
	}
	if stat.Uid != st.UID() {
		c.Fatal("got invalid uid")
	}
	if stat.Gid != st.GID() {
		c.Fatal("got invalid gid")
	}
	if stat.Rdev != st.Rdev() {
		c.Fatal("got invalid rdev")
	}
	if stat.Mtim != st.Mtim() {
		c.Fatal("got invalid mtim")
	}
}
