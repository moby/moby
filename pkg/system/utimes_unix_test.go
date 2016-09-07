// +build linux freebsd

package system

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/go-check/check"
)

// prepareFiles creates files for testing in the temp directory
func prepareFiles(c *check.C) (string, string, string, string) {
	dir, err := ioutil.TempDir("", "docker-system-test")
	if err != nil {
		c.Fatal(err)
	}

	file := filepath.Join(dir, "exist")
	if err := ioutil.WriteFile(file, []byte("hello"), 0644); err != nil {
		c.Fatal(err)
	}

	invalid := filepath.Join(dir, "doesnt-exist")

	symlink := filepath.Join(dir, "symlink")
	if err := os.Symlink(file, symlink); err != nil {
		c.Fatal(err)
	}

	return file, invalid, symlink, dir
}

func (s *DockerSuite) TestLUtimesNano(c *check.C) {
	file, invalid, symlink, dir := prepareFiles(c)
	defer os.RemoveAll(dir)

	before, err := os.Stat(file)
	if err != nil {
		c.Fatal(err)
	}

	ts := []syscall.Timespec{{Sec: 0, Nsec: 0}, {Sec: 0, Nsec: 0}}
	if err := LUtimesNano(symlink, ts); err != nil {
		c.Fatal(err)
	}

	symlinkInfo, err := os.Lstat(symlink)
	if err != nil {
		c.Fatal(err)
	}
	if before.ModTime().Unix() == symlinkInfo.ModTime().Unix() {
		c.Fatal("The modification time of the symlink should be different")
	}

	fileInfo, err := os.Stat(file)
	if err != nil {
		c.Fatal(err)
	}
	if before.ModTime().Unix() != fileInfo.ModTime().Unix() {
		c.Fatal("The modification time of the file should be same")
	}

	if err := LUtimesNano(invalid, ts); err == nil {
		c.Fatal("Doesn't return an error on a non-existing file")
	}
}
