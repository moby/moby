//go:build linux || freebsd
// +build linux freebsd

package meminfo

import (
	"strings"
	"testing"
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

	const KiB = 1024

	meminfo, err := parseMemInfo(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if meminfo.MemTotal != 1*KiB {
		t.Fatalf("Unexpected MemTotal: %d", meminfo.MemTotal)
	}
	if meminfo.MemFree != 3*KiB {
		t.Fatalf("Unexpected MemFree: %d", meminfo.MemFree)
	}
	if meminfo.SwapTotal != 4*KiB {
		t.Fatalf("Unexpected SwapTotal: %d", meminfo.SwapTotal)
	}
	if meminfo.SwapFree != 5*KiB {
		t.Fatalf("Unexpected SwapFree: %d", meminfo.SwapFree)
	}
}
