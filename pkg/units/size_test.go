package units

import (
	"strings"
	"testing"
)

func TestHumanSize(t *testing.T) {

	size := strings.Trim(HumanSize(1000), " \t")
	expect := "1 kB"
	if size != expect {
		t.Errorf("1000 -> expected '%s', got '%s'", expect, size)
	}

	size = strings.Trim(HumanSize(1024), " \t")
	expect = "1.024 kB"
	if size != expect {
		t.Errorf("1024 -> expected '%s', got '%s'", expect, size)
	}
}

func TestFromHumanSize(t *testing.T) {
	assertFromHumanSize(t, "32", false, 32)
	assertFromHumanSize(t, "32b", false, 32)
	assertFromHumanSize(t, "32B", false, 32)
	assertFromHumanSize(t, "32k", false, 32*1000)
	assertFromHumanSize(t, "32K", false, 32*1000)
	assertFromHumanSize(t, "32kb", false, 32*1000)
	assertFromHumanSize(t, "32Kb", false, 32*1000)
	assertFromHumanSize(t, "32Mb", false, 32*1000*1000)
	assertFromHumanSize(t, "32Gb", false, 32*1000*1000*1000)
	assertFromHumanSize(t, "32Tb", false, 32*1000*1000*1000*1000)
	assertFromHumanSize(t, "8Pb", false, 8*1000*1000*1000*1000*1000)

	assertFromHumanSize(t, "", true, -1)
	assertFromHumanSize(t, "hello", true, -1)
	assertFromHumanSize(t, "-32", true, -1)
	assertFromHumanSize(t, " 32 ", true, -1)
	assertFromHumanSize(t, "32 mb", true, -1)
	assertFromHumanSize(t, "32m b", true, -1)
	assertFromHumanSize(t, "32bm", true, -1)
}

func assertFromHumanSize(t *testing.T, size string, expectError bool, expectedBytes int64) {
	actualBytes, err := FromHumanSize(size)
	if (err != nil) && !expectError {
		t.Errorf("Unexpected error parsing '%s': %s", size, err)
	}
	if (err == nil) && expectError {
		t.Errorf("Expected to get an error parsing '%s', but got none (bytes=%d)", size, actualBytes)
	}
	if actualBytes != expectedBytes {
		t.Errorf("Expected '%s' to parse as %d bytes, got %d", size, expectedBytes, actualBytes)
	}
}

func TestRAMInBytes(t *testing.T) {
	assertRAMInBytes(t, "32", false, 32)
	assertRAMInBytes(t, "32b", false, 32)
	assertRAMInBytes(t, "32B", false, 32)
	assertRAMInBytes(t, "32k", false, 32*1024)
	assertRAMInBytes(t, "32K", false, 32*1024)
	assertRAMInBytes(t, "32kb", false, 32*1024)
	assertRAMInBytes(t, "32Kb", false, 32*1024)
	assertRAMInBytes(t, "32Mb", false, 32*1024*1024)
	assertRAMInBytes(t, "32Gb", false, 32*1024*1024*1024)

	assertRAMInBytes(t, "", true, -1)
	assertRAMInBytes(t, "hello", true, -1)
	assertRAMInBytes(t, "-32", true, -1)
	assertRAMInBytes(t, " 32 ", true, -1)
	assertRAMInBytes(t, "32 mb", true, -1)
	assertRAMInBytes(t, "32m b", true, -1)
	assertRAMInBytes(t, "32bm", true, -1)
}

func assertRAMInBytes(t *testing.T, size string, expectError bool, expectedBytes int64) {
	actualBytes, err := RAMInBytes(size)
	if (err != nil) && !expectError {
		t.Errorf("Unexpected error parsing '%s': %s", size, err)
	}
	if (err == nil) && expectError {
		t.Errorf("Expected to get an error parsing '%s', but got none (bytes=%d)", size, actualBytes)
	}
	if actualBytes != expectedBytes {
		t.Errorf("Expected '%s' to parse as %d bytes, got %d", size, expectedBytes, actualBytes)
	}
}
