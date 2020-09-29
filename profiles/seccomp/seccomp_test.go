// +build linux

package seccomp // import "github.com/docker/docker/profiles/seccomp"

import (
	"io/ioutil"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestLoadProfile(t *testing.T) {
	f, err := ioutil.ReadFile("fixtures/example.json")
	if err != nil {
		t.Fatal(err)
	}
	rs := createSpec()
	if _, err := LoadProfile(string(f), &rs); err != nil {
		t.Fatal(err)
	}
}

// TestLoadLegacyProfile tests loading a seccomp profile in the old format
// (before https://github.com/docker/docker/pull/24510)
func TestLoadLegacyProfile(t *testing.T) {
	f, err := ioutil.ReadFile("fixtures/default-old-format.json")
	if err != nil {
		t.Fatal(err)
	}
	rs := createSpec()
	if _, err := LoadProfile(string(f), &rs); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDefaultProfile(t *testing.T) {
	f, err := ioutil.ReadFile("default.json")
	if err != nil {
		t.Fatal(err)
	}
	rs := createSpec()
	if _, err := LoadProfile(string(f), &rs); err != nil {
		t.Fatal(err)
	}
}

func TestLoadConditional(t *testing.T) {
	f, err := ioutil.ReadFile("fixtures/conditional_include.json")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		doc      string
		cap      string
		expected []string
	}{
		{doc: "no caps", expected: []string{"chmod", "ptrace"}},
		{doc: "with syslog", cap: "CAP_SYSLOG", expected: []string{"chmod", "syslog", "ptrace"}},
		{doc: "no ptrace", cap: "CAP_SYS_ADMIN", expected: []string{"chmod"}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			rs := createSpec(tc.cap)
			p, err := LoadProfile(string(f), &rs)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Syscalls) != len(tc.expected) {
				t.Fatalf("expected %d syscalls in profile, have %d", len(tc.expected), len(p.Syscalls))
			}
			for i, v := range p.Syscalls {
				if v.Names[0] != tc.expected[i] {
					t.Fatalf("expected %s syscall, have %s", tc.expected[i], v.Names[0])
				}
			}
		})
	}
}

// createSpec() creates a minimum spec for testing
func createSpec(caps ...string) specs.Spec {
	rs := specs.Spec{
		Process: &specs.Process{
			Capabilities: &specs.LinuxCapabilities{},
		},
	}
	if caps != nil {
		rs.Process.Capabilities.Bounding = append(rs.Process.Capabilities.Bounding, caps...)
	}
	return rs
}
