package runconfig

import (
	"testing"

	"github.com/docker/docker/pkg/parsers"
)

func TestParseLxcConfOpt(t *testing.T) {
	opts := []string{"lxc.utsname=docker", "lxc.utsname = docker "}

	for _, o := range opts {
		k, v, err := parsers.ParseKeyValueOpt(o)
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

func TestNetHostname(t *testing.T) {
	if _, _, _, err := Parse([]string{"-h=name", "img", "cmd"}, nil); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if _, _, _, err := Parse([]string{"--net=host", "img", "cmd"}, nil); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if _, _, _, err := Parse([]string{"-h=name", "--net=bridge", "img", "cmd"}, nil); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if _, _, _, err := Parse([]string{"-h=name", "--net=none", "img", "cmd"}, nil); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if _, _, _, err := Parse([]string{"-h=name", "--net=host", "img", "cmd"}, nil); err != ErrConflictNetworkHostname {
		t.Fatalf("Expected error ErrConflictNetworkHostname, got: %s", err)
	}

	if _, _, _, err := Parse([]string{"-h=name", "--net=container:other", "img", "cmd"}, nil); err != ErrConflictNetworkHostname {
		t.Fatalf("Expected error ErrConflictNetworkHostname, got: %s", err)
	}
}
