package tarsum // import "github.com/docker/docker/pkg/tarsum"

import (
	"archive/tar"
	"fmt"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVersionLabelForChecksum(t *testing.T) {
	version := VersionLabelForChecksum("tarsum+sha256:deadbeef")
	if version != "tarsum" {
		t.Fatalf("Version should have been 'tarsum', was %v", version)
	}
	version = VersionLabelForChecksum("tarsum.v1+sha256:deadbeef")
	if version != "tarsum.v1" {
		t.Fatalf("Version should have been 'tarsum.v1', was %v", version)
	}
	version = VersionLabelForChecksum("something+somethingelse")
	if version != "something" {
		t.Fatalf("Version should have been 'something', was %v", version)
	}
	version = VersionLabelForChecksum("invalidChecksum")
	if version != "" {
		t.Fatalf("Version should have been empty, was %v", version)
	}
}

func TestVersion(t *testing.T) {
	expected := "tarsum"
	var v Version
	if v.String() != expected {
		t.Errorf("expected %q, got %q", expected, v.String())
	}

	expected = "tarsum.v1"
	v = 1
	if v.String() != expected {
		t.Errorf("expected %q, got %q", expected, v.String())
	}

	expected = "tarsum.dev"
	v = 2
	if v.String() != expected {
		t.Errorf("expected %q, got %q", expected, v.String())
	}
}

func TestGetVersion(t *testing.T) {
	testSet := []struct {
		Str      string
		Expected Version
	}{
		{"tarsum+sha256:e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b", Version0},
		{"tarsum+sha256", Version0},
		{"tarsum", Version0},
		{"tarsum.dev", VersionDev},
		{"tarsum.dev+sha256:deadbeef", VersionDev},
	}

	for _, ts := range testSet {
		v, err := GetVersionFromTarsum(ts.Str)
		if err != nil {
			t.Fatalf("%q : %s", err, ts.Str)
		}
		if v != ts.Expected {
			t.Errorf("expected %d (%q), got %d (%q)", ts.Expected, ts.Expected, v, v)
		}
	}

	// test one that does not exist, to ensure it errors
	str := "weak+md5:abcdeabcde"
	_, err := GetVersionFromTarsum(str)
	if err != ErrNotVersion {
		t.Fatalf("%q : %s", err, str)
	}
}

func TestGetVersions(t *testing.T) {
	expected := []Version{
		Version0,
		Version1,
		VersionDev,
	}
	versions := GetVersions()
	if len(versions) != len(expected) {
		t.Fatalf("Expected %v versions, got %v", len(expected), len(versions))
	}
	if !containsVersion(versions, expected[0]) || !containsVersion(versions, expected[1]) || !containsVersion(versions, expected[2]) {
		t.Fatalf("Expected [%v], got [%v]", expected, versions)
	}
}

func containsVersion(versions []Version, version Version) bool {
	for _, v := range versions {
		if v == version {
			return true
		}
	}
	return false
}

func TestSelectXattrsV1(t *testing.T) {
	hdr := &tar.Header{
		Xattrs: map[string]string{ //nolint:staticcheck
			"user.xattronly": "x",
			"user.foo":       "xattr",
		},
		PAXRecords: map[string]string{
			"SCHILY.xattr.user.paxonly": "p",
			"SCHILY.xattr.user.foo":     "paxrecord",
		},
	}
	selected := v1TarHeaderSelect(hdr)

	var s strings.Builder
	for _, elem := range selected {
		fmt.Fprintf(&s, "%s=%s\n", elem[0], elem[1])
	}
	t.Logf("Selected headers:\n%s", s.String())

	assert.Check(t, is.DeepEqual(selected[len(selected)-3:], [][2]string{
		{"user.foo", "xattr"},
		{"user.paxonly", "p"},
		{"user.xattronly", "x"},
	}))
}
