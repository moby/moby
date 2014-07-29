package fs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/libcontainer/cgroups"
)

const (
	cgroupFile  = "cgroup.file"
	floatValue  = 2048.0
	floatString = "2048"
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
	err = ioutil.WriteFile(tempFile, []byte(floatString), 0755)
	if err != nil {
		t.Fatal(err)
	}
	value, err := getCgroupParamInt(tempDir, cgroupFile)
	if err != nil {
		t.Fatal(err)
	} else if value != floatValue {
		t.Fatalf("Expected %d to equal %f", value, floatValue)
	}

	// Success with new line.
	err = ioutil.WriteFile(tempFile, []byte(floatString+"\n"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	value, err = getCgroupParamInt(tempDir, cgroupFile)
	if err != nil {
		t.Fatal(err)
	} else if value != floatValue {
		t.Fatalf("Expected %d to equal %f", value, floatValue)
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

func TestAbsolutePathHandling(t *testing.T) {
	testCgroup := cgroups.Cgroup{
		Name:   "bar",
		Parent: "/foo",
	}
	cgroupData := data{
		root:   "/sys/fs/cgroup",
		cgroup: "/foo/bar",
		c:      &testCgroup,
		pid:    1,
	}
	expectedPath := filepath.Join(cgroupData.root, "cpu", testCgroup.Parent, testCgroup.Name)
	if path, err := cgroupData.path("cpu"); path != expectedPath || err != nil {
		t.Fatalf("expected path %s but got %s %s", expectedPath, path, err)
	}
}
