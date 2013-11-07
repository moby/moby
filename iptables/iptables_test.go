package iptables

import (
	"os"
	"testing"
)

func TestIptables(t *testing.T) {
	if _, err := Raw("-L"); err != nil {
		t.Fatal(err)
	}
	path := os.Getenv("PATH")
	os.Setenv("PATH", "")
	defer os.Setenv("PATH", path)
	if _, err := Raw("-L"); err == nil {
		t.Fatal("Not finding iptables in the PATH should cause an error")
	}
}
