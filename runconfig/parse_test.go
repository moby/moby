package runconfig

import (
	"testing"

	"github.com/dotcloud/docker/utils"
)

func TestParseLxcConfOpt(t *testing.T) {
	opts := []string{"lxc.utsname=docker", "lxc.utsname = docker "}

	for _, o := range opts {
		k, v, err := utils.ParseKeyValueOpt(o)
		if err != nil {
			t.FailNow()
		}
		if k != "lxc.utsname" {
			t.Fail()
		}
		if v != "docker" {
			t.Fail()
		}
	}
}

func TestParseNetMode(t *testing.T) {
	testFlags := []struct {
		flag      string
		mode      string
		container string
		err       bool
	}{
		{"", "", "", true},
		{"bridge", "bridge", "", false},
		{"disable", "disable", "", false},
		{"container:foo", "container", "foo", false},
		{"container:", "", "", true},
		{"container", "", "", true},
		{"unknown", "", "", true},
	}

	for _, to := range testFlags {
		mode, err := parseNetMode(to.flag)
		if mode != to.mode {
			t.Fatalf("-net %s: expected net mode: %q, got: %q", to.flag, to.mode, mode)
		}
		if container != to.container {
			t.Fatalf("-net %s: expected net container: %q, got: %q", to.flag, to.container, container)
		}
		if (err != nil) != to.err {
			t.Fatal("-net %s: expected an error got none", to.flag)
		}
	}
}
