package seccomp

import (
	"fmt"
	"testing"
)

func TestGetKernelVersion(t *testing.T) {
	version, err := getKernelVersion()
	if err != nil {
		t.Fatal(err)
	}
	if version == nil {
		t.Fatal("version is nil")
	}
	if version.Kernel == 0 {
		t.Fatal("no kernel version")
	}
}

// TestParseRelease tests the ParseRelease() function
func TestParseRelease(t *testing.T) {
	tests := []struct {
		in          string
		out         KernelVersion
		expectedErr error
	}{
		{in: "3.8", out: KernelVersion{Kernel: 3, Major: 8}},
		{in: "3.8.0", out: KernelVersion{Kernel: 3, Major: 8}},
		{in: "3.8.0-19-generic", out: KernelVersion{Kernel: 3, Major: 8}},
		{in: "3.4.54.longterm-1", out: KernelVersion{Kernel: 3, Major: 4}},
		{in: "3.10.0-862.2.3.el7.x86_64", out: KernelVersion{Kernel: 3, Major: 10}},
		{in: "3.12.8tag", out: KernelVersion{Kernel: 3, Major: 12}},
		{in: "3.12-1-amd64", out: KernelVersion{Kernel: 3, Major: 12}},
		{in: "3.12foobar", out: KernelVersion{Kernel: 3, Major: 12}},
		{in: "99.999.999-19-generic", out: KernelVersion{Kernel: 99, Major: 999}},
		{in: "", expectedErr: fmt.Errorf(`failed to parse kernel version "": EOF`)},
		{in: "3", expectedErr: fmt.Errorf(`failed to parse kernel version "3": unexpected EOF`)},
		{in: "3.", expectedErr: fmt.Errorf(`failed to parse kernel version "3.": EOF`)},
		{in: "3a", expectedErr: fmt.Errorf(`failed to parse kernel version "3a": input does not match format`)},
		{in: "3.a", expectedErr: fmt.Errorf(`failed to parse kernel version "3.a": expected integer`)},
		{in: "a", expectedErr: fmt.Errorf(`failed to parse kernel version "a": expected integer`)},
		{in: "a.a", expectedErr: fmt.Errorf(`failed to parse kernel version "a.a": expected integer`)},
		{in: "a.a.a-a", expectedErr: fmt.Errorf(`failed to parse kernel version "a.a.a-a": expected integer`)},
		{in: "-3", expectedErr: fmt.Errorf(`failed to parse kernel version "-3": expected integer`)},
		{in: "-3.", expectedErr: fmt.Errorf(`failed to parse kernel version "-3.": expected integer`)},
		{in: "-3.8", expectedErr: fmt.Errorf(`failed to parse kernel version "-3.8": expected integer`)},
		{in: "-3.-8", expectedErr: fmt.Errorf(`failed to parse kernel version "-3.-8": expected integer`)},
		{in: "3.-8", expectedErr: fmt.Errorf(`failed to parse kernel version "3.-8": expected integer`)},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			version, err := parseRelease(tc.in)
			if tc.expectedErr != nil {
				if err == nil {
					t.Fatal("expected an error")
				}
				if err.Error() != tc.expectedErr.Error() {
					t.Fatalf("expected: %s, got: %s", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal("unexpected error:", err)
			}
			if version == nil {
				t.Fatal("version is nil")
			}
			if version.Kernel != tc.out.Kernel || version.Major != tc.out.Major {
				t.Fatalf("expected: %d.%d, got: %d.%d", tc.out.Kernel, tc.out.Major, version.Kernel, version.Major)
			}
		})
	}
}

func TestKernelGreaterEqualThan(t *testing.T) {
	// Get the current kernel version, so that we can make test relative to that
	v, err := getKernelVersion()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		doc      string
		in       KernelVersion
		expected bool
	}{
		{
			doc:      "same version",
			in:       KernelVersion{v.Kernel, v.Major},
			expected: true,
		},
		{
			doc:      "kernel minus one",
			in:       KernelVersion{v.Kernel - 1, v.Major},
			expected: true,
		},
		{
			doc:      "kernel plus one",
			in:       KernelVersion{v.Kernel + 1, v.Major},
			expected: false,
		},
		{
			doc:      "major plus one",
			in:       KernelVersion{v.Kernel, v.Major + 1},
			expected: false,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.doc+": "+tc.in.String(), func(t *testing.T) {
			ok, err := kernelGreaterEqualThan(tc.in)
			if err != nil {
				t.Fatal("unexpected error:", err)
			}
			if ok != tc.expected {
				t.Fatalf("expected: %v, got: %v", tc.expected, ok)
			}
		})
	}
}
