package tarsum

import "github.com/go-check/check"

func (s *DockerSuite) TestVersionLabelForChecksum(c *check.C) {
	version := VersionLabelForChecksum("tarsum+sha256:deadbeef")
	if version != "tarsum" {
		c.Fatalf("Version should have been 'tarsum', was %v", version)
	}
	version = VersionLabelForChecksum("tarsum.v1+sha256:deadbeef")
	if version != "tarsum.v1" {
		c.Fatalf("Version should have been 'tarsum.v1', was %v", version)
	}
	version = VersionLabelForChecksum("something+somethingelse")
	if version != "something" {
		c.Fatalf("Version should have been 'something', was %v", version)
	}
	version = VersionLabelForChecksum("invalidChecksum")
	if version != "" {
		c.Fatalf("Version should have been empty, was %v", version)
	}
}

func (s *DockerSuite) TestVersion(c *check.C) {
	expected := "tarsum"
	var v Version
	if v.String() != expected {
		c.Errorf("expected %q, got %q", expected, v.String())
	}

	expected = "tarsum.v1"
	v = 1
	if v.String() != expected {
		c.Errorf("expected %q, got %q", expected, v.String())
	}

	expected = "tarsum.dev"
	v = 2
	if v.String() != expected {
		c.Errorf("expected %q, got %q", expected, v.String())
	}
}

func (s *DockerSuite) TestGetVersion(c *check.C) {
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
			c.Fatalf("%q : %s", err, ts.Str)
		}
		if v != ts.Expected {
			c.Errorf("expected %d (%q), got %d (%q)", ts.Expected, ts.Expected, v, v)
		}
	}

	// test one that does not exist, to ensure it errors
	str := "weak+md5:abcdeabcde"
	_, err := GetVersionFromTarsum(str)
	if err != ErrNotVersion {
		c.Fatalf("%q : %s", err, str)
	}
}

func (s *DockerSuite) TestGetVersions(c *check.C) {
	expected := []Version{
		Version0,
		Version1,
		VersionDev,
	}
	versions := GetVersions()
	if len(versions) != len(expected) {
		c.Fatalf("Expected %v versions, got %v", len(expected), len(versions))
	}
	if !containsVersion(versions, expected[0]) || !containsVersion(versions, expected[1]) || !containsVersion(versions, expected[2]) {
		c.Fatalf("Expected [%v], got [%v]", expected, versions)
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
