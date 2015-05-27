// +build !experimental

package daemon

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/volume"
	"github.com/docker/docker/volume/drivers"
	"github.com/docker/docker/volume/local"
)

func TestGetVolumeDefaultDriver(t *testing.T) {
	tmp, err := ioutil.TempDir("", "volume-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	l, err := local.New(tmp)
	if err != nil {
		t.Fatal(err)
	}
	volumedrivers.Register(l, volume.DefaultDriverName)
	d, err := getVolumeDriver("missing")
	if err != nil {
		t.Fatal(err)
	}

	if d.Name() != volume.DefaultDriverName {
		t.Fatalf("Expected local driver, was %s\n", d.Name)
	}
}

func TestParseBindMount(t *testing.T) {
	cases := []struct {
		bind       string
		expDest    string
		expSource  string
		expName    string
		mountLabel string
		expRW      bool
		fail       bool
	}{
		{"/tmp:/tmp", "/tmp", "/tmp", "", "", true, false},
		{"/tmp:/tmp:ro", "/tmp", "/tmp", "", "", false, false},
		{"/tmp:/tmp:rw", "/tmp", "/tmp", "", "", true, false},
		{"/tmp:/tmp:foo", "/tmp", "/tmp", "", "", false, true},
		{"name:/tmp", "", "", "", "", false, true},
		{"local/name:/tmp:rw", "", "", "", "", true, true},
	}

	for _, c := range cases {
		conf := &runconfig.Config{}
		m, err := parseBindMount(c.bind, c.mountLabel, conf)
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

		if m.RW != c.expRW {
			t.Fatalf("Expected RW %v, was %v for spec %s\n", c.expRW, m.RW, c.bind)
		}
	}
}
