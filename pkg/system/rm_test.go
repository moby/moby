//go:build !darwin && !windows
// +build !darwin,!windows

package system // import "github.com/docker/docker/pkg/system"

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moby/sys/mount"
	"gotest.tools/v3/skip"
)

func TestEnsureRemoveAllWithMount(t *testing.T) {
	skip.If(t, os.Getuid() != 0, "skipping test that requires root")

	dir1, err := os.MkdirTemp("", "test-ensure-removeall-with-dir1")
	if err != nil {
		t.Fatal(err)
	}
	dir2, err := os.MkdirTemp("", "test-ensure-removeall-with-dir2")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir2)

	bindDir := filepath.Join(dir1, "bind")
	if err := os.MkdirAll(bindDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := mount.Mount(dir2, bindDir, "none", "bind"); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{}, 1)
	go func() {
		err = EnsureRemoveAll(dir1)
		close(done)
	}()

	select {
	case <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for EnsureRemoveAll to finish")
	}

	if _, err := os.Stat(dir1); !os.IsNotExist(err) {
		t.Fatalf("expected %q to not exist", dir1)
	}
}
