package dockerignore

import (
	"fmt"
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

func (s *DockerSuite) TestReadAll(c *check.C) {
	tmpDir, err := ioutil.TempDir("", "dockerignore-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	di, err := ReadAll(nil)
	if err != nil {
		c.Fatalf("Expected not to have error, got %v", err)
	}

	if diLen := len(di); diLen != 0 {
		c.Fatalf("Expected to have zero dockerignore entry, got %d", diLen)
	}

	diName := filepath.Join(tmpDir, ".dockerignore")
	content := fmt.Sprintf("test1\n/test2\n/a/file/here\n\nlastfile")
	err = ioutil.WriteFile(diName, []byte(content), 0777)
	if err != nil {
		c.Fatal(err)
	}

	diFd, err := os.Open(diName)
	if err != nil {
		c.Fatal(err)
	}
	defer diFd.Close()

	di, err = ReadAll(diFd)
	if err != nil {
		c.Fatal(err)
	}

	if di[0] != "test1" {
		c.Fatalf("First element is not test1")
	}
	if di[1] != "/test2" {
		c.Fatalf("Second element is not /test2")
	}
	if di[2] != "/a/file/here" {
		c.Fatalf("Third element is not /a/file/here")
	}
	if di[3] != "lastfile" {
		c.Fatalf("Fourth element is not lastfile")
	}
}
