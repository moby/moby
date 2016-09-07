// +build !windows

package mount

import (
	"os"
	"path"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestMountOptionsParsing(c *check.C) {
	options := "noatime,ro,size=10k"

	flag, data := parseOptions(options)

	if data != "size=10k" {
		c.Fatalf("Expected size=10 got %s", data)
	}

	expectedFlag := NOATIME | RDONLY

	if flag != expectedFlag {
		c.Fatalf("Expected %d got %d", expectedFlag, flag)
	}
}

func (s *DockerSuite) TestMounted(c *check.C) {
	tmp := path.Join(os.TempDir(), "mount-tests")
	if err := os.MkdirAll(tmp, 0777); err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	var (
		sourceDir  = path.Join(tmp, "source")
		targetDir  = path.Join(tmp, "target")
		sourcePath = path.Join(sourceDir, "file.txt")
		targetPath = path.Join(targetDir, "file.txt")
	)

	os.Mkdir(sourceDir, 0777)
	os.Mkdir(targetDir, 0777)

	f, err := os.Create(sourcePath)
	if err != nil {
		c.Fatal(err)
	}
	f.WriteString("hello")
	f.Close()

	f, err = os.Create(targetPath)
	if err != nil {
		c.Fatal(err)
	}
	f.Close()

	if err := Mount(sourceDir, targetDir, "none", "bind,rw"); err != nil {
		c.Fatal(err)
	}
	defer func() {
		if err := Unmount(targetDir); err != nil {
			c.Fatal(err)
		}
	}()

	mounted, err := Mounted(targetDir)
	if err != nil {
		c.Fatal(err)
	}
	if !mounted {
		c.Fatalf("Expected %s to be mounted", targetDir)
	}
	if _, err := os.Stat(targetDir); err != nil {
		c.Fatal(err)
	}
}

func (s *DockerSuite) TestMountReadonly(c *check.C) {
	tmp := path.Join(os.TempDir(), "mount-tests")
	if err := os.MkdirAll(tmp, 0777); err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	var (
		sourceDir  = path.Join(tmp, "source")
		targetDir  = path.Join(tmp, "target")
		sourcePath = path.Join(sourceDir, "file.txt")
		targetPath = path.Join(targetDir, "file.txt")
	)

	os.Mkdir(sourceDir, 0777)
	os.Mkdir(targetDir, 0777)

	f, err := os.Create(sourcePath)
	if err != nil {
		c.Fatal(err)
	}
	f.WriteString("hello")
	f.Close()

	f, err = os.Create(targetPath)
	if err != nil {
		c.Fatal(err)
	}
	f.Close()

	if err := Mount(sourceDir, targetDir, "none", "bind,ro"); err != nil {
		c.Fatal(err)
	}
	defer func() {
		if err := Unmount(targetDir); err != nil {
			c.Fatal(err)
		}
	}()

	f, err = os.OpenFile(targetPath, os.O_RDWR, 0777)
	if err == nil {
		c.Fatal("Should not be able to open a ro file as rw")
	}
}

func (s *DockerSuite) TestGetMounts(c *check.C) {
	mounts, err := GetMounts()
	if err != nil {
		c.Fatal(err)
	}

	root := false
	for _, entry := range mounts {
		if entry.Mountpoint == "/" {
			root = true
		}
	}

	if !root {
		c.Fatal("/ should be mounted at least")
	}
}

func (s *DockerSuite) TestMergeTmpfsOptions(c *check.C) {
	options := []string{"noatime", "ro", "size=10k", "defaults", "atime", "defaults", "rw", "rprivate", "size=1024k", "slave"}
	expected := []string{"atime", "rw", "size=1024k", "slave"}
	merged, err := MergeTmpfsOptions(options)
	if err != nil {
		c.Fatal(err)
	}
	if len(expected) != len(merged) {
		c.Fatalf("Expected %s got %s", expected, merged)
	}
	for index := range merged {
		if merged[index] != expected[index] {
			c.Fatalf("Expected %s for the %dth option, got %s", expected, index, merged)
		}
	}

	options = []string{"noatime", "ro", "size=10k", "atime", "rw", "rprivate", "size=1024k", "slave", "size"}
	_, err = MergeTmpfsOptions(options)
	if err == nil {
		c.Fatal("Expected error got nil")
	}
}
