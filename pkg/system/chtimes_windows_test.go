// +build windows

package system

import (
	"os"
	"syscall"
	"testing"
	"time"
)

// TestChtimes tests Chtimes access time on a tempfile on Windows
func TestChtimesWindows(t *testing.T) {
	file, dir := prepareTempFile(t)
	defer os.RemoveAll(dir)

	beforeUnixEpochTime := time.Unix(0, 0).Add(-100 * time.Second)
	unixEpochTime := time.Unix(0, 0)
	afterUnixEpochTime := time.Unix(100, 0)
	// The max Unix time is 33 bits set
	unixMaxTime := unixEpochTime.Add((1<<33 - 1) * time.Second)
	afterUnixMaxTime := unixMaxTime.Add(100 * time.Second)

	// Test both aTime and mTime set to Unix Epoch
	Chtimes(file, unixEpochTime, unixEpochTime)

	f, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	aTime := time.Unix(0, f.Sys().(*syscall.Win32FileAttributeData).LastAccessTime.Nanoseconds())
	if aTime != unixEpochTime {
		t.Fatalf("Expected: %s, got: %s", unixEpochTime, aTime)
	}

	// Test aTime before Unix Epoch and mTime set to Unix Epoch
	Chtimes(file, beforeUnixEpochTime, unixEpochTime)

	f, err = os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	aTime = time.Unix(0, f.Sys().(*syscall.Win32FileAttributeData).LastAccessTime.Nanoseconds())
	if aTime != unixEpochTime {
		t.Fatalf("Expected: %s, got: %s", unixEpochTime, aTime)
	}

	// Test aTime set to Unix Epoch and mTime before Unix Epoch
	Chtimes(file, unixEpochTime, beforeUnixEpochTime)

	f, err = os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	aTime = time.Unix(0, f.Sys().(*syscall.Win32FileAttributeData).LastAccessTime.Nanoseconds())
	if aTime != unixEpochTime {
		t.Fatalf("Expected: %s, got: %s", unixEpochTime, aTime)
	}

	// Test both aTime and mTime set to after Unix Epoch (valid time)
	Chtimes(file, afterUnixEpochTime, afterUnixEpochTime)

	f, err = os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	aTime = time.Unix(0, f.Sys().(*syscall.Win32FileAttributeData).LastAccessTime.Nanoseconds())
	if aTime != afterUnixEpochTime {
		t.Fatalf("Expected: %s, got: %s", afterUnixEpochTime, aTime)
	}

	// Test both aTime and mTime set to Unix max time
	Chtimes(file, unixMaxTime, unixMaxTime)

	f, err = os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	aTime = time.Unix(0, f.Sys().(*syscall.Win32FileAttributeData).LastAccessTime.Nanoseconds())
	if aTime != unixMaxTime {
		t.Fatalf("Expected: %s, got: %s", unixMaxTime, aTime)
	}

	// Test aTime after Unix max time and mTime set to Unix max time
	Chtimes(file, afterUnixMaxTime, unixMaxTime)

	f, err = os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	aTime = time.Unix(0, f.Sys().(*syscall.Win32FileAttributeData).LastAccessTime.Nanoseconds())
	if aTime != unixEpochTime {
		t.Fatalf("Expected: %s, got: %s", unixEpochTime, aTime)
	}

	// Test aTime set to Unix Epoch and mTime before Unix Epoch
	Chtimes(file, unixMaxTime, afterUnixMaxTime)

	f, err = os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	aTime = time.Unix(0, f.Sys().(*syscall.Win32FileAttributeData).LastAccessTime.Nanoseconds())
	if aTime != unixMaxTime {
		t.Fatalf("Expected: %s, got: %s", unixMaxTime, aTime)
	}
}
