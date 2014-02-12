package runconfig

import (
	"testing"
)

func TestParseLxcConfOpt(t *testing.T) {
	opts := []string{"lxc.utsname=docker", "lxc.utsname = docker "}

	for _, o := range opts {
		k, v, err := parseLxcOpt(o)
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
