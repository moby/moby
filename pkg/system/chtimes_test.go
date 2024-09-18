package system // import "github.com/docker/docker/pkg/system"

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestChtimesModTime tests Chtimes on a tempfile. Test only mTime, because
// aTime is OS dependent.
func TestChtimesModTime(t *testing.T) {
	file := filepath.Join(t.TempDir(), "exist")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	beforeUnixEpochTime := unixEpochTime.Add(-100 * time.Second)
	afterUnixEpochTime := unixEpochTime.Add(100 * time.Second)

	// Test both aTime and mTime set to Unix Epoch
	t.Run("both aTime and mTime set to Unix Epoch", func(t *testing.T) {
		if err := Chtimes(file, unixEpochTime, unixEpochTime); err != nil {
			t.Error(err)
		}

		f, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}

		if f.ModTime() != unixEpochTime {
			t.Fatalf("Expected: %s, got: %s", unixEpochTime, f.ModTime())
		}
	})

	// Test aTime before Unix Epoch and mTime set to Unix Epoch
	t.Run("aTime before Unix Epoch and mTime set to Unix Epoch", func(t *testing.T) {
		if err := Chtimes(file, beforeUnixEpochTime, unixEpochTime); err != nil {
			t.Error(err)
		}

		f, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}

		if f.ModTime() != unixEpochTime {
			t.Fatalf("Expected: %s, got: %s", unixEpochTime, f.ModTime())
		}
	})

	// Test aTime set to Unix Epoch and mTime before Unix Epoch
	t.Run("aTime set to Unix Epoch and mTime before Unix Epoch", func(t *testing.T) {
		if err := Chtimes(file, unixEpochTime, beforeUnixEpochTime); err != nil {
			t.Error(err)
		}

		f, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}

		if f.ModTime() != unixEpochTime {
			t.Fatalf("Expected: %s, got: %s", unixEpochTime, f.ModTime())
		}
	})

	// Test both aTime and mTime set to after Unix Epoch (valid time)
	t.Run("both aTime and mTime set to after Unix Epoch (valid time)", func(t *testing.T) {
		if err := Chtimes(file, afterUnixEpochTime, afterUnixEpochTime); err != nil {
			t.Error(err)
		}

		f, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}

		if f.ModTime() != afterUnixEpochTime {
			t.Fatalf("Expected: %s, got: %s", afterUnixEpochTime, f.ModTime())
		}
	})

	// Test both aTime and mTime set to Unix max time
	t.Run("both aTime and mTime set to Unix max time", func(t *testing.T) {
		if err := Chtimes(file, unixMaxTime, unixMaxTime); err != nil {
			t.Error(err)
		}

		f, err := os.Stat(file)
		if err != nil {
			t.Fatal(err)
		}

		if f.ModTime().Truncate(time.Second) != unixMaxTime.Truncate(time.Second) {
			t.Fatalf("Expected: %s, got: %s", unixMaxTime.Truncate(time.Second), f.ModTime().Truncate(time.Second))
		}
	})
}
