//go:build linux || freebsd
// +build linux freebsd

package system // import "github.com/docker/docker/pkg/system"

import (
	"strings"
	"testing"

	units "github.com/docker/go-units"
)

// TestMemInfo tests parseMemInfo with a static meminfo string
func TestMemInfo(t *testing.T) {
	const input = `
	MemTotal:      1 kB
	MemFree:       2 kB
	MemAvailable:  3 kB
	SwapTotal:     4 kB
	SwapFree:      5 kB
	Malformed1:
	Malformed2:    1
	Malformed3:    2 MB
	Malformed4:    X kB
	`
	meminfo, err := parseMemInfo(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if meminfo.MemTotal != 1*units.KiB {
		t.Fatalf("Unexpected MemTotal: %d", meminfo.MemTotal)
	}
	if meminfo.MemFree != 3*units.KiB {
		t.Fatalf("Unexpected MemFree: %d", meminfo.MemFree)
	}
	if meminfo.SwapTotal != 4*units.KiB {
		t.Fatalf("Unexpected SwapTotal: %d", meminfo.SwapTotal)
	}
	if meminfo.SwapFree != 5*units.KiB {
		t.Fatalf("Unexpected SwapFree: %d", meminfo.SwapFree)
	}
}
