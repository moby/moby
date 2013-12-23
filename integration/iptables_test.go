package docker

import (
	"github.com/dotcloud/docker/pkg/iptables"
	"os"
	"testing"
)

// FIXME: this test should be a unit test.
// For example by mocking os/exec to make sure iptables is not actually called.

func TestIptables(t *testing.T) {
	if _, err := iptables.Raw("-L"); err != nil {
		t.Fatal(err)
	}
	path := os.Getenv("PATH")
	os.Setenv("PATH", "")
	defer os.Setenv("PATH", path)
	if _, err := iptables.Raw("-L"); err == nil {
		t.Fatal("Not finding iptables in the PATH should cause an error")
	}
}
