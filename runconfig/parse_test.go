package runconfig

import (
	"io/ioutil"
	"testing"

	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
	"github.com/docker/docker/pkg/sysinfo"
)

func parseRun(args []string, sysInfo *sysinfo.SysInfo) (*Config, *HostConfig, *flag.FlagSet, error) {
	cmd := flag.NewFlagSet("run", flag.ContinueOnError)
	cmd.SetOutput(ioutil.Discard)
	cmd.Usage = nil
	return Parse(cmd, args, sysInfo)
}

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
	if _, _, _, err := parseRun([]string{"-h=name", "img", "cmd"}, nil); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if _, _, _, err := parseRun([]string{"--net=host", "img", "cmd"}, nil); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if _, _, _, err := parseRun([]string{"-h=name", "--net=bridge", "img", "cmd"}, nil); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if _, _, _, err := parseRun([]string{"-h=name", "--net=none", "img", "cmd"}, nil); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if _, _, _, err := parseRun([]string{"-h=name", "--net=host", "img", "cmd"}, nil); err != ErrConflictNetworkHostname {
		t.Fatalf("Expected error ErrConflictNetworkHostname, got: %s", err)
	}

	if _, _, _, err := parseRun([]string{"-h=name", "--net=container:other", "img", "cmd"}, nil); err != ErrConflictNetworkHostname {
		t.Fatalf("Expected error ErrConflictNetworkHostname, got: %s", err)
	}
}
