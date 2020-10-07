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
	if version.kernel == 0 {
		t.Fatal("no kernel version")
	}
}

// TestParseRelease tests the ParseRelease() function
func TestParseRelease(t *testing.T) {
	tests := []struct {
		in          string
		out         kernelVersion
		expectedErr error
	}{
		{in: "3.8", out: kernelVersion{kernel: 3, major: 8}},
		{in: "3.8.0", out: kernelVersion{kernel: 3, major: 8}},
		{in: "3.8.0-19-generic", out: kernelVersion{kernel: 3, major: 8}},
		{in: "3.4.54.longterm-1", out: kernelVersion{kernel: 3, major: 4}},
		{in: "3.10.0-862.2.3.el7.x86_64", out: kernelVersion{kernel: 3, major: 10}},
		{in: "3.12.8tag", out: kernelVersion{kernel: 3, major: 12}},
		{in: "3.12-1-amd64", out: kernelVersion{kernel: 3, major: 12}},
		{in: "3.12foobar", out: kernelVersion{kernel: 3, major: 12}},
		{in: "99.999.999-19-generic", out: kernelVersion{kernel: 99, major: 999}},
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
			if version.kernel != tc.out.kernel || version.major != tc.out.major {
				t.Fatalf("expected: %d.%d, got: %d.%d", tc.out.kernel, tc.out.major, version.kernel, version.major)
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
		in       string
		expected bool
	}{
		{
			doc:      "same version",
			in:       fmt.Sprintf("%d.%d", v.kernel, v.major),
			expected: true,
		},
		{
			doc:      "kernel minus one",
			in:       fmt.Sprintf("%d.%d", v.kernel-1, v.major),
			expected: true,
		},
		{
			doc:      "kernel plus one",
			in:       fmt.Sprintf("%d.%d", v.kernel+1, v.major),
			expected: false,
		},
		{
			doc:      "major plus one",
			in:       fmt.Sprintf("%d.%d", v.kernel, v.major+1),
			expected: false,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.doc+": "+tc.in, func(t *testing.T) {
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
