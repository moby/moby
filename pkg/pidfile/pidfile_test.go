package pidfile

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestNewAndRemove(c *check.C) {
	dir, err := ioutil.TempDir(os.TempDir(), "test-pidfile")
	if err != nil {
		c.Fatal("Could not create test directory")
	}

	path := filepath.Join(dir, "testfile")
	file, err := New(path)
	if err != nil {
		c.Fatal("Could not create test file", err)
	}

	_, err = New(path)
	if err == nil {
		c.Fatal("Test file creation not blocked")
	}

	if err := file.Remove(); err != nil {
		c.Fatal("Could not delete created test file")
	}
}

func (s *DockerSuite) TestRemoveInvalidPath(c *check.C) {
	file := PIDFile{path: filepath.Join("foo", "bar")}

	if err := file.Remove(); err == nil {
		c.Fatal("Non-existing file doesn't give an error on delete")
	}
}
