package system // import "github.com/docker/docker/pkg/system"

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestChtimesLinux tests Chtimes access time on a tempfile on Linux
func TestChtimesLinux(t *testing.T) {
	file := filepath.Join(t.TempDir(), "exist")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	beforeUnixEpochTime := unixEpochTime.Add(-100 * time.Second)
	afterUnixEpochTime := unixEpochTime.Add(100 * time.Second)

	// Test both aTime and mTime set to Unix Epoch
	Chtimes(file, unixEpochTime, unixEpochTime)

	f, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	stat := f.Sys().(*syscall.Stat_t)
	aTime := time.Unix(stat.Atim.Unix())
	if aTime != unixEpochTime {
		t.Fatalf("Expected: %s, got: %s", unixEpochTime, aTime)
	}

	// Test aTime before Unix Epoch and mTime set to Unix Epoch
	Chtimes(file, beforeUnixEpochTime, unixEpochTime)

	f, err = os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	stat = f.Sys().(*syscall.Stat_t)
	aTime = time.Unix(stat.Atim.Unix())
	if aTime != unixEpochTime {
		t.Fatalf("Expected: %s, got: %s", unixEpochTime, aTime)
	}

	// Test aTime set to Unix Epoch and mTime before Unix Epoch
	Chtimes(file, unixEpochTime, beforeUnixEpochTime)

	f, err = os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	stat = f.Sys().(*syscall.Stat_t)
	aTime = time.Unix(stat.Atim.Unix())
	if aTime != unixEpochTime {
		t.Fatalf("Expected: %s, got: %s", unixEpochTime, aTime)
	}

	// Test both aTime and mTime set to after Unix Epoch (valid time)
	Chtimes(file, afterUnixEpochTime, afterUnixEpochTime)

	f, err = os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	stat = f.Sys().(*syscall.Stat_t)
	aTime = time.Unix(stat.Atim.Unix())
	if aTime != afterUnixEpochTime {
		t.Fatalf("Expected: %s, got: %s", afterUnixEpochTime, aTime)
	}

	// Test both aTime and mTime set to Unix max time
	Chtimes(file, unixMaxTime, unixMaxTime)

	f, err = os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	stat = f.Sys().(*syscall.Stat_t)
	aTime = time.Unix(stat.Atim.Unix())
	if aTime.Truncate(time.Second) != unixMaxTime.Truncate(time.Second) {
		t.Fatalf("Expected: %s, got: %s", unixMaxTime.Truncate(time.Second), aTime.Truncate(time.Second))
	}
}
