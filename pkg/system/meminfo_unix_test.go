// +build linux freebsd

package system

import (
	"strings"

	units "src/github.com/docker/go-units"

	"github.com/go-check/check"
)

// TestMemInfo tests parseMemInfo with a static meminfo string
func (s *DockerSuite) TestMemInfo(c *check.C) {
	const input = `
	MemTotal:      1 kB
	MemFree:       2 kB
	SwapTotal:     3 kB
	SwapFree:      4 kB
	Malformed1:
	Malformed2:    1
	Malformed3:    2 MB
	Malformed4:    X kB
	`
	meminfo, err := parseMemInfo(strings.NewReader(input))
	if err != nil {
		c.Fatal(err)
	}
	if meminfo.MemTotal != 1*units.KiB {
		c.Fatalf("Unexpected MemTotal: %d", meminfo.MemTotal)
	}
	if meminfo.MemFree != 2*units.KiB {
		c.Fatalf("Unexpected MemFree: %d", meminfo.MemFree)
	}
	if meminfo.SwapTotal != 3*units.KiB {
		c.Fatalf("Unexpected SwapTotal: %d", meminfo.SwapTotal)
	}
	if meminfo.SwapFree != 4*units.KiB {
		c.Fatalf("Unexpected SwapFree: %d", meminfo.SwapFree)
	}
}
