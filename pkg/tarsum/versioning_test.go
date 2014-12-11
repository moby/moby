package tarsum

import (
	"testing"
)

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
