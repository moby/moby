package sysinfo

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestReadProcBool(c *check.C) {
	tmpDir, err := ioutil.TempDir("", "test-sysinfo-proc")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	procFile := filepath.Join(tmpDir, "read-proc-bool")
	if err := ioutil.WriteFile(procFile, []byte("1"), 644); err != nil {
		c.Fatal(err)
	}

	if !readProcBool(procFile) {
		c.Fatal("expected proc bool to be true, got false")
	}

	if err := ioutil.WriteFile(procFile, []byte("0"), 644); err != nil {
		c.Fatal(err)
	}
	if readProcBool(procFile) {
		c.Fatal("expected proc bool to be false, got false")
	}

	if readProcBool(path.Join(tmpDir, "no-exist")) {
		c.Fatal("should be false for non-existent entry")
	}

}

func (s *DockerSuite) TestCgroupEnabled(c *check.C) {
	cgroupDir, err := ioutil.TempDir("", "cgroup-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(cgroupDir)

	if cgroupEnabled(cgroupDir, "test") {
		c.Fatal("cgroupEnabled should be false")
	}

	if err := ioutil.WriteFile(path.Join(cgroupDir, "test"), []byte{}, 644); err != nil {
		c.Fatal(err)
	}

	if !cgroupEnabled(cgroupDir, "test") {
		c.Fatal("cgroupEnabled should be true")
	}
}
