package system // import "github.com/docker/docker/pkg/system"

import (
	"os"
	"syscall"
	"testing"
	"time"
)

var (
	testData = []struct {
		effectValue, expectedValue time.Time
	}{
		{time.Unix(0, 0).Add(-100 * time.Second), time.Unix(0, 0)},
		{time.Unix(0, 0), time.Unix(0, 0)},
		{time.Unix(100, 0), time.Unix(100, 0)},
	}
)

// TestChtimesLinux tests Chtimes access time on a tempfile on Linux
func TestChtimesLinux(t *testing.T) {
	file, dir := prepareTempFile(t)
	defer os.RemoveAll(dir)

	beforeUnixEpochTime := time.Unix(0, 0).Add(-100 * time.Second)
	unixEpochTime := time.Unix(0, 0)
	afterUnixEpochTime := time.Unix(100, 0)
	unixMaxTime := maxTime

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

// TestChtimesNoFollowLinux tests ChtimesNoFollow access time on a tempfile and its symbol link on Linux
func TestChtimesNoFollowLinux(t *testing.T) {
	file, dir := prepareTempFile(t)
	link := file + ".lnk"
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// test realfile
	for _, aTimeData := range testData {
		for _, mTimeData := range testData {
			if aTimeData.effectValue.Before(mTimeData.effectValue) {
				continue
			}
			ChtimesNoFollow(file, aTimeData.effectValue, mTimeData.effectValue)
			Chtimes(file, aTimeData.effectValue, mTimeData.effectValue)
			f, err := os.Lstat(file)
			if err != nil {
				t.Fatal(err)
			}
			stat := f.Sys().(*syscall.Stat_t)
			aTime := time.Unix(int64(stat.Atim.Sec), int64(stat.Atim.Nsec)) //nolint: unconvert
			if aTime != aTimeData.expectedValue {
				t.Fatalf("Expected: %s after set %s, got: %s", aTimeData.expectedValue, aTimeData.effectValue, aTime)
			}
		}
	}

	// test linkfile
	for _, aTimeData := range testData {
		for _, mTimeData := range testData {
			if aTimeData.effectValue.Before(mTimeData.effectValue) {
				continue
			}
			ChtimesNoFollow(link, aTimeData.effectValue, mTimeData.effectValue)
			f, err := os.Lstat(link)
			if err != nil {
				t.Fatal(err)
			}
			stat := f.Sys().(*syscall.Stat_t)
			aTime := time.Unix(int64(stat.Atim.Sec), int64(stat.Atim.Nsec)) //nolint: unconvert
			if aTime != aTimeData.expectedValue {
				t.Fatalf("Symbol link, expected: %s after set %s, got: %s", aTimeData.expectedValue, aTimeData.effectValue, aTime)
			}
		}
	}

	// test link and real files
	for _, realfileTimeData := range testData {
		for _, linkfileTimeData := range testData {
			ChtimesNoFollow(file, realfileTimeData.effectValue, realfileTimeData.effectValue)
			ChtimesNoFollow(link, linkfileTimeData.effectValue, linkfileTimeData.effectValue)
			f, err := os.Lstat(file)
			if err != nil {
				t.Fatal(err)
			}
			stat := f.Sys().(*syscall.Stat_t)
			aTime := time.Unix(int64(stat.Atim.Sec), int64(stat.Atim.Nsec)) //nolint: unconvert
			if aTime != realfileTimeData.expectedValue {
				t.Fatalf("Realfile, expected: %s after set %s, got: %s", realfileTimeData.expectedValue, realfileTimeData.effectValue, aTime)
			}
			f, err = os.Lstat(link)
			if err != nil {
				t.Fatal(err)
			}
			stat = f.Sys().(*syscall.Stat_t)
			aTime = time.Unix(int64(stat.Atim.Sec), int64(stat.Atim.Nsec)) //nolint: unconvert
			if aTime != linkfileTimeData.expectedValue {
				t.Fatalf("Linkfile, expected: %s after set %s, got: %s", linkfileTimeData.expectedValue, linkfileTimeData.effectValue, aTime)
			}
		}
	}
}
