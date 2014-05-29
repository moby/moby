package fs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

const (
	cgroupFile  = "cgroup.file"
	int64Value  = 2048
	int64String = "2048"
)

func TestGetCgroupParamsInt(t *testing.T) {
	// Setup tempdir.
	tempDir, err := ioutil.TempDir("", "cgroup_utils_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	tempFile := filepath.Join(tempDir, cgroupFile)

	// Success.
	err = ioutil.WriteFile(tempFile, []byte(int64String), 0755)
	if err != nil {
		t.Fatal(err)
	}
	value, err := getCgroupParamInt(tempDir, cgroupFile)
	if err != nil {
		t.Fatal(err)
	} else if value != int64Value {
		t.Fatalf("Expected %f to equal %f", value, int64String)
	}

	// Success with new line.
	err = ioutil.WriteFile(tempFile, []byte(int64String+"\n"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	value, err = getCgroupParamInt(tempDir, cgroupFile)
	if err != nil {
		t.Fatal(err)
	} else if value != int64Value {
		t.Fatalf("Expected %f to equal %f", value, int64Value)
	}

	// Not a float.
	err = ioutil.WriteFile(tempFile, []byte("not-a-float"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	_, err = getCgroupParamInt(tempDir, cgroupFile)
	if err == nil {
		t.Fatal("Expecting error, got none")
	}

	// Unknown file.
	err = os.Remove(tempFile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = getCgroupParamInt(tempDir, cgroupFile)
	if err == nil {
		t.Fatal("Expecting error, got none")
	}
}
