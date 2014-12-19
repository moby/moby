// +build linux

package btrfs

import (
	"testing"
)

func TestBuildVersion(t *testing.T) {
	if len(BtrfsBuildVersion()) == 0 {
		t.Errorf("expected output from btrfs build version, but got empty string")
	}
}
