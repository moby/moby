// +build linux,!btrfs_noversion

package btrfs

import (
	"testing"
)

func TestLibVersion(t *testing.T) {
	if btrfsLibVersion() <= 0 {
		t.Errorf("expected output from btrfs lib version > 0")
	}
}
