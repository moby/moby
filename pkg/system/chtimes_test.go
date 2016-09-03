package system

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

// prepareTempFile creates a temporary file in a temporary directory.
func prepareTempFile(c *check.C) (string, string) {
	dir, err := ioutil.TempDir("", "docker-system-test")
	if err != nil {
		c.Fatal(err)
	}

	file := filepath.Join(dir, "exist")
	if err := ioutil.WriteFile(file, []byte("hello"), 0644); err != nil {
		c.Fatal(err)
	}
	return file, dir
}

// TestChtimes tests Chtimes on a tempfile. Test only mTime, because aTime is OS dependent
func (s *DockerSuite) TestChtimes(c *check.C) {
	file, dir := prepareTempFile(c)
	defer os.RemoveAll(dir)

	beforeUnixEpochTime := time.Unix(0, 0).Add(-100 * time.Second)
	unixEpochTime := time.Unix(0, 0)
	afterUnixEpochTime := time.Unix(100, 0)
	unixMaxTime := maxTime

	// Test both aTime and mTime set to Unix Epoch
	Chtimes(file, unixEpochTime, unixEpochTime)

	f, err := os.Stat(file)
	if err != nil {
		c.Fatal(err)
	}

	if f.ModTime() != unixEpochTime {
		c.Fatalf("Expected: %s, got: %s", unixEpochTime, f.ModTime())
	}

	// Test aTime before Unix Epoch and mTime set to Unix Epoch
	Chtimes(file, beforeUnixEpochTime, unixEpochTime)

	f, err = os.Stat(file)
	if err != nil {
		c.Fatal(err)
	}

	if f.ModTime() != unixEpochTime {
		c.Fatalf("Expected: %s, got: %s", unixEpochTime, f.ModTime())
	}

	// Test aTime set to Unix Epoch and mTime before Unix Epoch
	Chtimes(file, unixEpochTime, beforeUnixEpochTime)

	f, err = os.Stat(file)
	if err != nil {
		c.Fatal(err)
	}

	if f.ModTime() != unixEpochTime {
		c.Fatalf("Expected: %s, got: %s", unixEpochTime, f.ModTime())
	}

	// Test both aTime and mTime set to after Unix Epoch (valid time)
	Chtimes(file, afterUnixEpochTime, afterUnixEpochTime)

	f, err = os.Stat(file)
	if err != nil {
		c.Fatal(err)
	}

	if f.ModTime() != afterUnixEpochTime {
		c.Fatalf("Expected: %s, got: %s", afterUnixEpochTime, f.ModTime())
	}

	// Test both aTime and mTime set to Unix max time
	Chtimes(file, unixMaxTime, unixMaxTime)

	f, err = os.Stat(file)
	if err != nil {
		c.Fatal(err)
	}

	if f.ModTime().Truncate(time.Second) != unixMaxTime.Truncate(time.Second) {
		c.Fatalf("Expected: %s, got: %s", unixMaxTime.Truncate(time.Second), f.ModTime().Truncate(time.Second))
	}
}
