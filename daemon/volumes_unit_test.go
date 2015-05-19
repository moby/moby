package daemon

import (
	"testing"

	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/volume"
	volumedrivers "github.com/docker/docker/volume/drivers"
)

func TestParseNamedVolumeInfo(t *testing.T) {
	cases := []struct {
		driver    string
		name      string
		expDriver string
		expName   string
	}{
		{"", "name", "local", "name"},
		{"external", "name", "external", "name"},
		{"", "external/name", "external", "name"},
		{"ignored", "external/name", "external", "name"},
	}

	for _, c := range cases {
		conf := &runconfig.Config{VolumeDriver: c.driver}
		driver, name := parseNamedVolumeInfo(c.name, conf)

		if driver != c.expDriver {
			t.Fatalf("Expected %s, was %s\n", c.expDriver, driver)
		}

		if name != c.expName {
			t.Fatalf("Expected %s, was %s\n", c.expName, name)
		}
	}
}

func TestParseBindMount(t *testing.T) {
	cases := []struct {
		bind      string
		driver    string
		expDest   string
		expSource string
		expName   string
		expDriver string
		expRW     bool
		fail      bool
	}{
		{"/tmp:/tmp", "", "/tmp", "/tmp", "", "", true, false},
		{"/tmp:/tmp:ro", "", "/tmp", "/tmp", "", "", false, false},
		{"/tmp:/tmp:rw", "", "/tmp", "/tmp", "", "", true, false},
		{"/tmp:/tmp:foo", "", "/tmp", "/tmp", "", "", false, true},
		{"name:/tmp", "", "", "", "", "", false, true},
		{"name:/tmp", "external", "/tmp", "", "name", "external", true, false},
		{"external/name:/tmp:rw", "", "/tmp", "", "name", "external", true, false},
		{"external/name:/tmp:ro", "", "/tmp", "", "name", "external", false, false},
		{"external/name:/tmp:foo", "", "/tmp", "", "name", "external", false, true},
		{"name:/tmp", "local", "", "", "", "", false, true},
		{"local/name:/tmp:rw", "", "", "", "", "", true, true},
	}

	for _, c := range cases {
		conf := &runconfig.Config{VolumeDriver: c.driver}
		m, err := parseBindMount(c.bind, conf)
		if c.fail {
			if err == nil {
				t.Fatalf("Expected error, was nil, for spec %s\n", c.bind)
			}
			continue
		}

		if m.Destination != c.expDest {
			t.Fatalf("Expected destination %s, was %s, for spec %s\n", c.expDest, m.Destination, c.bind)
		}

		if m.Source != c.expSource {
			t.Fatalf("Expected source %s, was %s, for spec %s\n", c.expSource, m.Source, c.bind)
		}

		if m.Name != c.expName {
			t.Fatalf("Expected name %s, was %s for spec %s\n", c.expName, m.Name, c.bind)
		}

		if m.Driver != c.expDriver {
			t.Fatalf("Expected driver %s, was %s, for spec %s\n", c.expDriver, m.Driver, c.bind)
		}

		if m.RW != c.expRW {
			t.Fatalf("Expected RW %v, was %v for spec %s\n", c.expRW, m.RW, c.bind)
		}
	}
}

func TestParseVolumeFrom(t *testing.T) {
	cases := []struct {
		spec    string
		expId   string
		expMode string
		fail    bool
	}{
		{"", "", "", true},
		{"foobar", "foobar", "rw", false},
		{"foobar:rw", "foobar", "rw", false},
		{"foobar:ro", "foobar", "ro", false},
		{"foobar:baz", "", "", true},
	}

	for _, c := range cases {
		id, mode, err := parseVolumesFrom(c.spec)
		if c.fail {
			if err == nil {
				t.Fatalf("Expected error, was nil, for spec %s\n", c.spec)
			}
			continue
		}

		if id != c.expId {
			t.Fatalf("Expected id %s, was %s, for spec %s\n", c.expId, id, c.spec)
		}
		if mode != c.expMode {
			t.Fatalf("Expected mode %s, was %s for spec %s\n", c.expMode, mode, c.spec)
		}
	}
}

type fakeDriver struct{}

func (fakeDriver) Name() string                              { return "fake" }
func (fakeDriver) Create(name string) (volume.Volume, error) { return nil, nil }
func (fakeDriver) Remove(v volume.Volume) error              { return nil }

func TestGetVolumeDriver(t *testing.T) {
	_, err := getVolumeDriver("missing")
	if err == nil {
		t.Fatal("Expected error, was nil")
	}

	volumedrivers.Register(fakeDriver{}, "fake")
	d, err := getVolumeDriver("fake")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name() != "fake" {
		t.Fatalf("Expected fake driver, got %s\n", d.Name())
	}
}
