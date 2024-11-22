//go:build !windows

package containerd

import (
	"os"
	"testing"
)

func checkRequirement(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requires root")
	}
}
