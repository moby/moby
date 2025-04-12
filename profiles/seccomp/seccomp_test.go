//go:build linux

package seccomp // import "github.com/docker/docker/profiles/seccomp"

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func assertDeepEqual(t *testing.T, expected interface{}, actual interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("\nexpected: %+#v\ngot     : %+#v", expected, actual)
	}
}

func TestLoadProfile(t *testing.T) {
	f, err := os.ReadFile("fixtures/example.json")
	if err != nil {
		t.Fatal(err)
	}
	rs := createSpec()
	p, err := LoadProfile(string(f), &rs)
	if err != nil {
		t.Fatal(err)
	}
	var expectedErrno uint = 12345
	var expectedDefaultErrno uint = 1
	expected := specs.LinuxSeccomp{
		DefaultAction:   specs.ActErrno,
		DefaultErrnoRet: &expectedDefaultErrno,
		Syscalls: []specs.LinuxSyscall{
			{
				Names:  []string{"clone"},
				Action: specs.ActAllow,
				Args: []specs.LinuxSeccompArg{{
					Index:    0,
					Value:    2114060288,
					ValueTwo: 0,
					Op:       specs.OpMaskedEqual,
				}},
			},
			{
				Names:  []string{"open"},
				Action: specs.ActAllow,
				Args:   []specs.LinuxSeccompArg{},
			},
			{
				Names:  []string{"close"},
				Action: specs.ActAllow,
				Args:   []specs.LinuxSeccompArg{},
			},
			{
				Names:    []string{"syslog"},
				Action:   specs.ActErrno,
				ErrnoRet: &expectedErrno,
				Args:     []specs.LinuxSeccompArg{},
			},
		},
	}

	assertDeepEqual(t, expected, *p)
}

func TestLoadProfileWithDefaultErrnoRet(t *testing.T) {
	profile := []byte(`{
"defaultAction": "SCMP_ACT_ERRNO",
"defaultErrnoRet": 6
}`)
	rs := createSpec()
	p, err := LoadProfile(string(profile), &rs)
	if err != nil {
		t.Fatal(err)
	}

	expectedErrnoRet := uint(6)
	expected := specs.LinuxSeccomp{
		DefaultAction:   specs.ActErrno,
		DefaultErrnoRet: &expectedErrnoRet,
	}

	assertDeepEqual(t, expected, *p)
}

func TestLoadProfileWithListenerPath(t *testing.T) {
	profile := []byte(`{
"defaultAction": "SCMP_ACT_ERRNO",
"listenerPath": "/var/run/seccompaget.sock",
"listenerMetadata": "opaque-metadata"
}`)
	rs := createSpec()
	p, err := LoadProfile(string(profile), &rs)
	if err != nil {
		t.Fatal(err)
	}

	expected := specs.LinuxSeccomp{
		DefaultAction:    specs.ActErrno,
		ListenerPath:     "/var/run/seccompaget.sock",
		ListenerMetadata: "opaque-metadata",
	}

	assertDeepEqual(t, expected, *p)
}

func TestLoadProfileWithFlag(t *testing.T) {
	profile := `{"defaultAction": "SCMP_ACT_ERRNO", "flags": ["SECCOMP_FILTER_FLAG_SPEC_ALLOW", "SECCOMP_FILTER_FLAG_LOG"]}`
	expected := specs.LinuxSeccomp{
		DefaultAction: specs.ActErrno,
		Flags:         []specs.LinuxSeccompFlag{"SECCOMP_FILTER_FLAG_SPEC_ALLOW", "SECCOMP_FILTER_FLAG_LOG"},
	}
	rs := createSpec()
	p, err := LoadProfile(profile, &rs)
	if err != nil {
		t.Fatal(err)
	}
	assertDeepEqual(t, expected, *p)
}

// TestLoadProfileValidation tests that invalid profiles produce the correct error.
func TestLoadProfileValidation(t *testing.T) {
	tests := []struct {
		doc      string
		profile  string
		expected string
	}{
		{
			doc:      "conflicting architectures and archMap",
			profile:  `{"defaultAction": "SCMP_ACT_ERRNO", "architectures": ["A", "B", "C"], "archMap": [{"architecture": "A", "subArchitectures": ["B", "C"]}]}`,
			expected: `both 'architectures' and 'archMap' are specified in the seccomp profile, use either 'architectures' or 'archMap'`,
		},
		{
			doc:      "conflicting syscall.name and syscall.names",
			profile:  `{"defaultAction": "SCMP_ACT_ERRNO", "syscalls": [{"name": "accept", "names": ["accept"], "action": "SCMP_ACT_ALLOW"}]}`,
			expected: `both 'name' and 'names' are specified in the seccomp profile, use either 'name' or 'names'`,
		},
	}
	for _, tc := range tests {
		rs := createSpec()
		t.Run(tc.doc, func(t *testing.T) {
			_, err := LoadProfile(tc.profile, &rs)
			if err == nil {
				t.Fatal("expected error")
			}
			if tc.expected != err.Error() {
				t.Fatalf("expected: %q, got: %q", tc.expected, err)
			}
		})
	}
}

// TestLoadLegacyProfile tests loading a seccomp profile in the old format
// (before https://github.com/docker/docker/pull/24510)
func TestLoadLegacyProfile(t *testing.T) {
	f, err := os.ReadFile("fixtures/default-old-format.json")
	if err != nil {
		t.Fatal(err)
	}
	rs := createSpec()
	p, err := LoadProfile(string(f), &rs)
	if err != nil {
		t.Fatal(err)
	}
	if p.DefaultAction != specs.ActErrno {
		t.Fatalf("expected default action %s, got %s", specs.ActErrno, p.DefaultAction)
	}
	expectedArches := []specs.Arch{"SCMP_ARCH_X86_64", "SCMP_ARCH_X86", "SCMP_ARCH_X32"}
	assertDeepEqual(t, expectedArches, p.Architectures)

	if expected := 311; len(p.Syscalls) != expected {
		t.Fatalf("expected %d syscalls, got %d", expected, len(p.Syscalls))
	}
	expected := specs.LinuxSyscall{
		Names:  []string{"accept"},
		Action: specs.ActAllow,
		Args:   []specs.LinuxSeccompArg{},
	}
	assertDeepEqual(t, expected, p.Syscalls[0])
}

func TestLoadDefaultProfile(t *testing.T) {
	f, err := os.ReadFile("default.json")
	if err != nil {
		t.Fatal(err)
	}
	rs := createSpec()
	if _, err := LoadProfile(string(f), &rs); err != nil {
		t.Fatal(err)
	}
}

func TestUnmarshalDefaultProfile(t *testing.T) {
	expected := DefaultProfile()
	if expected == nil {
		t.Skip("seccomp not supported")
	}

	f, err := os.ReadFile("default.json")
	if err != nil {
		t.Fatal(err)
	}
	var profile Seccomp
	err = json.Unmarshal(f, &profile)
	if err != nil {
		t.Fatal(err)
	}
	assertDeepEqual(t, expected.Architectures, profile.Architectures)
	assertDeepEqual(t, expected.ArchMap, profile.ArchMap)
	assertDeepEqual(t, expected.DefaultAction, profile.DefaultAction)
	assertDeepEqual(t, expected.Syscalls, profile.Syscalls)
}

func TestMarshalUnmarshalFilter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in    string
		out   string
		error bool
	}{
		{in: `{"arches":["s390x"],"minKernel":3}`, error: true},
		{in: `{"arches":["s390x"],"minKernel":3.12}`, error: true},
		{in: `{"arches":["s390x"],"minKernel":true}`, error: true},
		{in: `{"arches":["s390x"],"minKernel":"0.0"}`, error: true},
		{in: `{"arches":["s390x"],"minKernel":"3"}`, error: true},
		{in: `{"arches":["s390x"],"minKernel":".3"}`, error: true},
		{in: `{"arches":["s390x"],"minKernel":"3."}`, error: true},
		{in: `{"arches":["s390x"],"minKernel":"true"}`, error: true},
		{in: `{"arches":["s390x"],"minKernel":"3.12.1\""}`, error: true},
		{in: `{"arches":["s390x"],"minKernel":"4.15abc"}`, error: true},
		{in: `{"arches":["s390x"],"minKernel":null}`, out: `{"arches":["s390x"]}`},
		{in: `{"arches":["s390x"],"minKernel":""}`, out: `{"arches":["s390x"],"minKernel":""}`}, // FIXME: try to fix omitempty for this
		{in: `{"arches":["s390x"],"minKernel":"0.5"}`, out: `{"arches":["s390x"],"minKernel":"0.5"}`},
		{in: `{"arches":["s390x"],"minKernel":"0.50"}`, out: `{"arches":["s390x"],"minKernel":"0.50"}`},
		{in: `{"arches":["s390x"],"minKernel":"5.0"}`, out: `{"arches":["s390x"],"minKernel":"5.0"}`},
		{in: `{"arches":["s390x"],"minKernel":"50.0"}`, out: `{"arches":["s390x"],"minKernel":"50.0"}`},
		{in: `{"arches":["s390x"],"minKernel":"4.15"}`, out: `{"arches":["s390x"],"minKernel":"4.15"}`},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			var filter Filter
			err := json.Unmarshal([]byte(tc.in), &filter)
			if tc.error {
				if err == nil {
					t.Fatal("expected an error")
				} else if !strings.Contains(err.Error(), "invalid kernel version") {
					t.Fatal("unexpected error:", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			out, err := json.Marshal(filter)
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != tc.out {
				t.Fatalf("expected %s, got %s", tc.out, string(out))
			}
		})
	}
}

func TestLoadConditional(t *testing.T) {
	f, err := os.ReadFile("fixtures/conditional_include.json")
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
