// +build linux

package seccomp

import (
	"io/ioutil"
	"testing"
)

func TestLoadProfile(t *testing.T) {
	f, err := ioutil.ReadFile("fixtures/example.json")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := LoadProfile(string(f)); err != nil {
		t.Fatal(err)
	}
}
