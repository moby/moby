package iptables

import (
	"os"
	"testing"
)

func TestIptables(t *testing.T) {
	if err := Raw("-L"); err != nil {
		t.Fatal(err)
	}
	path := os.Getenv("PATH")
	os.Setenv("PATH", "")
	defer os.Setenv("PATH", path)
	if err := Raw("-L"); err == nil {
		t.Fatal("Not finding iptables in the PATH should cause an error")
	}
}
