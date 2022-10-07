package system // import "github.com/docker/docker/pkg/system"

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// prepareTempFile creates a temporary file in a temporary directory.
func prepareTempFile(t *testing.T) (string, string) {
	dir, err := os.MkdirTemp("", "docker-system-test")
	if err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(dir, "exist")
	if err := os.WriteFile(file, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	return file, dir
}

// TestChtimes tests Chtimes on a tempfile. Test only mTime, because aTime is OS dependent
func TestChtimes(t *testing.T) {
	file, dir := prepareTempFile(t)
	defer os.RemoveAll(dir)

	beforeUnixEpochTime := unixEpochTime.Add(-100 * time.Second)
	afterUnixEpochTime := unixEpochTime.Add(100 * time.Second)

	// Test both aTime and mTime set to Unix Epoch
	Chtimes(file, unixEpochTime, unixEpochTime)

	f, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	if f.ModTime() != unixEpochTime {
		t.Fatalf("Expected: %s, got: %s", unixEpochTime, f.ModTime())
	}

	// Test aTime before Unix Epoch and mTime set to Unix Epoch
	Chtimes(file, beforeUnixEpochTime, unixEpochTime)

	f, err = os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	if f.ModTime() != unixEpochTime {
		t.Fatalf("Expected: %s, got: %s", unixEpochTime, f.ModTime())
	}

	// Test aTime set to Unix Epoch and mTime before Unix Epoch
	Chtimes(file, unixEpochTime, beforeUnixEpochTime)

	f, err = os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	if f.ModTime() != unixEpochTime {
		t.Fatalf("Expected: %s, got: %s", unixEpochTime, f.ModTime())
	}

	// Test both aTime and mTime set to after Unix Epoch (valid time)
	Chtimes(file, afterUnixEpochTime, afterUnixEpochTime)

	f, err = os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	if f.ModTime() != afterUnixEpochTime {
		t.Fatalf("Expected: %s, got: %s", afterUnixEpochTime, f.ModTime())
	}

	// Test both aTime and mTime set to Unix max time
	Chtimes(file, unixMaxTime, unixMaxTime)

	f, err = os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	if f.ModTime().Truncate(time.Second) != unixMaxTime.Truncate(time.Second) {
		t.Fatalf("Expected: %s, got: %s", unixMaxTime.Truncate(time.Second), f.ModTime().Truncate(time.Second))
	}
}
