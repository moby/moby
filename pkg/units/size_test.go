package units

import (
	"testing"
)

func TestHumanSize(t *testing.T) {
	assertEquals(t, "1 kB", HumanSize(1000))
	assertEquals(t, "1.024 kB", HumanSize(1024))
	assertEquals(t, "1 MB", HumanSize(1000000))
	assertEquals(t, "1.049 MB", HumanSize(1048576))
	assertEquals(t, "2 MB", HumanSize(2*1000*1000))
	assertEquals(t, "3.42 GB", HumanSize(3.42*1000*1000*1000))
	assertEquals(t, "5.372 TB", HumanSize(5.372*1000*1000*1000*1000))
	assertEquals(t, "2.22 PB", HumanSize(2.22*1000*1000*1000*1000*1000))
	assertEquals(t, "2.22 EB", HumanSize(2.22*1000*1000*1000*1000*1000*1000))
	assertEquals(t, "7.707 EB", HumanSize(7.707*1000*1000*1000*1000*1000*1000))
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
	assertRAMInBytes(t, "32MB", false, 32*1024*1024)
	assertRAMInBytes(t, "32Gb", false, 32*1024*1024*1024)
	assertRAMInBytes(t, "32G", false, 32*1024*1024*1024)
	assertRAMInBytes(t, "32Tb", false, 32*1024*1024*1024*1024)

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
