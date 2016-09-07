// +build linux

package btrfs

import (
	"os"
	"path"
	"testing"

	"github.com/docker/docker/daemon/graphdriver/graphtest"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

// This avoids creating a new driver for each test if all tests are run
// Make sure to put new tests between TestBtrfsSetup and TestBtrfsTeardown
func (s *DockerSuite) TestBtrfsSetup(c *check.C) {
	graphtest.GetDriver(c, "btrfs")
}

func (s *DockerSuite) TestBtrfsCreateEmpty(c *check.C) {
	graphtest.DriverTestCreateEmpty(c, "btrfs")
}

func (s *DockerSuite) TestBtrfsCreateBase(c *check.C) {
	graphtest.DriverTestCreateBase(c, "btrfs")
}

func (s *DockerSuite) TestBtrfsCreateSnap(c *check.C) {
	graphtest.DriverTestCreateSnap(c, "btrfs")
}

func (s *DockerSuite) TestBtrfsSubvolDelete(c *check.C) {
	d := graphtest.GetDriver(c, "btrfs")
	if err := d.CreateReadWrite("test", "", "", nil); err != nil {
		c.Fatal(err)
	}
	defer graphtest.PutDriver(c)

	dir, err := d.Get("test", "")
	if err != nil {
		c.Fatal(err)
	}
	defer d.Put("test")

	if err := subvolCreate(dir, "subvoltest"); err != nil {
		c.Fatal(err)
	}

	if _, err := os.Stat(path.Join(dir, "subvoltest")); err != nil {
		c.Fatal(err)
	}

	if err := d.Remove("test"); err != nil {
		c.Fatal(err)
	}

	if _, err := os.Stat(path.Join(dir, "subvoltest")); !os.IsNotExist(err) {
		c.Fatalf("expected not exist error on nested subvol, got: %v", err)
	}
}

func (s *DockerSuite) TestBtrfsTeardown(c *check.C) {
	graphtest.PutDriver(c)
}
