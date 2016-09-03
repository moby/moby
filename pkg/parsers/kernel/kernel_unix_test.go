// +build !windows

package kernel

import (
	"fmt"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func assertParseRelease(c *check.C, release string, b *VersionInfo, result int) {
	var (
		a *VersionInfo
	)
	a, _ = ParseRelease(release)

	if r := CompareKernelVersion(*a, *b); r != result {
		c.Fatalf("Unexpected kernel version comparison result for (%v,%v). Found %d, expected %d", release, b, r, result)
	}
	if a.Flavor != b.Flavor {
		c.Fatalf("Unexpected parsed kernel flavor.  Found %s, expected %s", a.Flavor, b.Flavor)
	}
}

// TestParseRelease tests the ParseRelease() function
func (s *DockerSuite) TestParseRelease(c *check.C) {
	assertParseRelease(c, "3.8.0", &VersionInfo{Kernel: 3, Major: 8, Minor: 0}, 0)
	assertParseRelease(c, "3.4.54.longterm-1", &VersionInfo{Kernel: 3, Major: 4, Minor: 54, Flavor: ".longterm-1"}, 0)
	assertParseRelease(c, "3.4.54.longterm-1", &VersionInfo{Kernel: 3, Major: 4, Minor: 54, Flavor: ".longterm-1"}, 0)
	assertParseRelease(c, "3.8.0-19-generic", &VersionInfo{Kernel: 3, Major: 8, Minor: 0, Flavor: "-19-generic"}, 0)
	assertParseRelease(c, "3.12.8tag", &VersionInfo{Kernel: 3, Major: 12, Minor: 8, Flavor: "tag"}, 0)
	assertParseRelease(c, "3.12-1-amd64", &VersionInfo{Kernel: 3, Major: 12, Minor: 0, Flavor: "-1-amd64"}, 0)
	assertParseRelease(c, "3.8.0", &VersionInfo{Kernel: 4, Major: 8, Minor: 0}, -1)
	// Errors
	invalids := []string{
		"3",
		"a",
		"a.a",
		"a.a.a-a",
	}
	for _, invalid := range invalids {
		expectedMessage := fmt.Sprintf("Can't parse kernel version %v", invalid)
		if _, err := ParseRelease(invalid); err == nil || err.Error() != expectedMessage {

		}
	}
}

func assertKernelVersion(c *check.C, a, b VersionInfo, result int) {
	if r := CompareKernelVersion(a, b); r != result {
		c.Fatalf("Unexpected kernel version comparison result. Found %d, expected %d", r, result)
	}
}

// TestCompareKernelVersion tests the CompareKernelVersion() function
func (s *DockerSuite) TestCompareKernelVersion(c *check.C) {
	assertKernelVersion(c,
		VersionInfo{Kernel: 3, Major: 8, Minor: 0},
		VersionInfo{Kernel: 3, Major: 8, Minor: 0},
		0)
	assertKernelVersion(c,
		VersionInfo{Kernel: 2, Major: 6, Minor: 0},
		VersionInfo{Kernel: 3, Major: 8, Minor: 0},
		-1)
	assertKernelVersion(c,
		VersionInfo{Kernel: 3, Major: 8, Minor: 0},
		VersionInfo{Kernel: 2, Major: 6, Minor: 0},
		1)
	assertKernelVersion(c,
		VersionInfo{Kernel: 3, Major: 8, Minor: 0},
		VersionInfo{Kernel: 3, Major: 8, Minor: 0},
		0)
	assertKernelVersion(c,
		VersionInfo{Kernel: 3, Major: 8, Minor: 5},
		VersionInfo{Kernel: 3, Major: 8, Minor: 0},
		1)
	assertKernelVersion(c,
		VersionInfo{Kernel: 3, Major: 0, Minor: 20},
		VersionInfo{Kernel: 3, Major: 8, Minor: 0},
		-1)
	assertKernelVersion(c,
		VersionInfo{Kernel: 3, Major: 7, Minor: 20},
		VersionInfo{Kernel: 3, Major: 8, Minor: 0},
		-1)
	assertKernelVersion(c,
		VersionInfo{Kernel: 3, Major: 8, Minor: 20},
		VersionInfo{Kernel: 3, Major: 7, Minor: 0},
		1)
	assertKernelVersion(c,
		VersionInfo{Kernel: 3, Major: 8, Minor: 20},
		VersionInfo{Kernel: 3, Major: 8, Minor: 0},
		1)
	assertKernelVersion(c,
		VersionInfo{Kernel: 3, Major: 8, Minor: 0},
		VersionInfo{Kernel: 3, Major: 8, Minor: 20},
		-1)
}
