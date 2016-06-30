package multiplatform

import "testing"

func TestIsAbsWindows(t *testing.T) {
	if !IsAbsWindows(`c:\windows`) {
		t.Fatal("windows path not abs")
	}
}

func TestIsAbsUnix(t *testing.T) {
	if !IsAbsUnix("/foo") {
		t.Fatal("unix path not abs")
	}
}

func TestVolumeNameWindows(t *testing.T) {
	if VolumeNameWindows(`c:\windows`) != "c:" {
		t.Fatal("no volume name")
	}

	if VolumeNameWindows("/foo") != "" {
		t.Fatal("got volume name")
	}
}
