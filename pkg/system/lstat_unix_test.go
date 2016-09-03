// +build linux freebsd

package system

import (
	"os"

	"github.com/go-check/check"
)

// TestLstat tests Lstat for existing and non existing files
func (s *DockerSuite) TestLstat(c *check.C) {
	file, invalid, _, dir := prepareFiles(c)
	defer os.RemoveAll(dir)

	statFile, err := Lstat(file)
	if err != nil {
		c.Fatal(err)
	}
	if statFile == nil {
		c.Fatal("returned empty stat for existing file")
	}

	statInvalid, err := Lstat(invalid)
	if err == nil {
		c.Fatal("did not return error for non-existing file")
	}
	if statInvalid != nil {
		c.Fatal("returned non-nil stat for non-existing file")
	}
}
