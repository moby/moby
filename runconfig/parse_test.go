package runconfig

import (
	"io/ioutil"
	"testing"

	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"
)

func parseRun(args []string) (*Config, *HostConfig, *flag.FlagSet, error) {
	cmd := flag.NewFlagSet("run", flag.ContinueOnError)
	cmd.SetOutput(ioutil.Discard)
	cmd.Usage = nil
	return Parse(cmd, args)
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
	if _, _, _, err := parseRun([]string{"-h=name", "img", "cmd"}); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if _, _, _, err := parseRun([]string{"--net=host", "img", "cmd"}); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if _, _, _, err := parseRun([]string{"-h=name", "--net=bridge", "img", "cmd"}); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if _, _, _, err := parseRun([]string{"-h=name", "--net=none", "img", "cmd"}); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if _, _, _, err := parseRun([]string{"-h=name", "--net=host", "img", "cmd"}); err != ErrConflictNetworkHostname {
		t.Fatalf("Expected error ErrConflictNetworkHostname, got: %s", err)
	}

	if _, _, _, err := parseRun([]string{"-h=name", "--net=container:other", "img", "cmd"}); err != ErrConflictNetworkHostname {
		t.Fatalf("Expected error ErrConflictNetworkHostname, got: %s", err)
	}
}

func TestConflictContainerNetworkAndLinks(t *testing.T) {
	if _, _, _, err := parseRun([]string{"--net=container:other", "--link=zip:zap", "img", "cmd"}); err != ErrConflictContainerNetworkAndLinks {
		t.Fatalf("Expected error ErrConflictContainerNetworkAndLinks, got: %s", err)
	}
}
