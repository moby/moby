//go:build linux

package oci

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/docker/docker/profiles/seccomp"
)

func TestSeccompLoadProfile(t *testing.T) {
	profiles := []string{"default.json", "default-old-format.json", "example.json"}

	for _, p := range profiles {
		t.Run(p, func(t *testing.T) {
			f, err := os.ReadFile("fixtures/" + p)
			if err != nil {
				t.Fatal(err)
			}
			rs := DefaultLinuxSpec()
			if _, err := seccomp.LoadProfile(string(f), &rs); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSeccompLoadDefaultProfile(t *testing.T) {
	b, err := json.Marshal(seccomp.DefaultProfile())
	if err != nil {
		t.Fatal(err)
	}
	rs := DefaultLinuxSpec()
	if _, err := seccomp.LoadProfile(string(b), &rs); err != nil {
		t.Fatal(err)
	}
}
