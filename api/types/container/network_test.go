package container

import (
	"encoding/json"
	"testing"
)

type TestRanger interface {
	Range() PortRange
}

var _ TestRanger = Port{}
var _ TestRanger = PortRange{}

func TestParsePort(t *testing.T) {
	tests := []struct {
		in        string
		port      Port      // output of ParsePort()
		str       string    // output of String(). If "", use in.
		portRange PortRange // output of Range()
	}{
		// Zero port
		{
			in:        "0/tcp",
			port:      Port{num: 0, proto: TCP},
			str:       "0/tcp",
			portRange: PortRange{start: 0, end: 0, proto: TCP},
		},
		// Max valid port
		{
			in:        "65535/tcp",
			port:      Port{num: 65535, proto: TCP},
			str:       "65535/tcp",
			portRange: PortRange{start: 65535, end: 65535, proto: TCP},
		},
		// Simple valid ports
		{
			in:        "1234/tcp",
			port:      Port{num: 1234, proto: TCP},
			str:       "1234/tcp",
			portRange: PortRange{start: 1234, end: 1234, proto: TCP},
		},
		{
			in:        "1234/udp",
			port:      Port{num: 1234, proto: UDP},
			str:       "1234/udp",
			portRange: PortRange{start: 1234, end: 1234, proto: UDP},
		},
		{
			in:        "1234/sctp",
			port:      Port{num: 1234, proto: SCTP},
			str:       "1234/sctp",
			portRange: PortRange{start: 1234, end: 1234, proto: SCTP},
		},
		// Default protocol is tcp
		{
			in:        "1234",
			port:      Port{num: 1234, proto: TCP},
			str:       "1234/tcp",
			portRange: PortRange{start: 1234, end: 1234, proto: TCP},
		},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParsePort(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.port {
				t.Errorf("expected port %+v, got %+v", tc.port, got)
			}

			MustParsePort(tc.in) // should not panic

			// Check that ParsePort is a pure function.
			if got2, err := ParsePort(tc.in); err != nil {
				t.Fatal(err)
			} else if got2 != got {
				t.Errorf("ParsePort(%q) got 2 different results %#v, %#v", tc.in, got, got2)
			}

			// Check that ParsePort(port.String()) is the identity function.
			if got3, err := ParsePort(got.String()); err != nil {
				t.Fatal(err)
			} else if got3 != got {
				t.Errorf("ParsePort(%q) != ParsePort(ParsePort(%q).String()) Got %#v, want %#v", tc.in, tc.in, got3, got)
			}

			// Check String() output
			s := got.String()
			wants := tc.str
			if wants == "" {
				wants = tc.in
			}
			if s != wants {
				t.Errorf("ParsePort(%q).String() got %q, want %q", tc.in, s, wants)
			}

			js := `"` + tc.in + `"`
			var jsgot Port
			if err := json.Unmarshal([]byte(js), &jsgot); err != nil {
				t.Fatal(err)
			}
			if jsgot != got {
				t.Errorf("json.Unmarshal(%q) = %#v, want %#v", tc.in, jsgot, got)
			}
			jsb, err := json.Marshal(jsgot)
			if err != nil {
				t.Fatal(err)
			}
			jswant := `"` + wants + `"`
			jsback := string(jsb)
			if jsback != jswant {
				t.Errorf("json.Marshal(json.Unmarshal(%q)) = %q, want %q", tc.in, jsback, jswant)
			}

			// Check Range() output
			if r := got.Range(); r != tc.portRange {
				t.Errorf("ParsePort(%q).Range() = %+v, want %+v", tc.in, r, tc.portRange)
			}
		})
	}

	negativeTests := []string{
		// Empty string
		"",
		// Whitespace-only string
		" ",
		// Negative port
		"-1",
		// Too large port
		"65536",
		// Non-numeric port
		"foo",
		// Port range instead of single port
		"1234-1240/udp",
		// Port range instead of single port without protocol
		"1234-1240",
		// Garbage port
		"asd1234/tcp",
	}

	for _, s := range negativeTests {
		t.Run(s, func(t *testing.T) {
			got, err := ParsePort(s)
			if err == nil {
				t.Errorf("ParsePort(%q), got port %+v", s, got)
			}

			var jsgot Port
			js := []byte(`"` + s + `"`)
			if err := json.Unmarshal(js, &jsgot); err == nil {
				t.Errorf("json.Unmarshal(%q) = %#v", s, jsgot)
			}
		})
	}
}

func TestPortRange(t *testing.T) {
	tests := []struct {
		in        string
		portRange PortRange // output of ParsePortRange() and Range()
		str       string    // output of String(). If "", use in.

	}{
		// Zero port
		{
			in:        "0-1234/tcp",
			portRange: PortRange{start: 0, end: 1234, proto: TCP},
			str:       "0-1234/tcp",
		},
		// Max valid port
		{
			in:        "1234-65535/tcp",
			portRange: PortRange{start: 1234, end: 65535, proto: TCP},
			str:       "1234-65535/tcp",
		},
		// Simple valid ports
		{
			in:        "1234-4567/tcp",
			portRange: PortRange{start: 1234, end: 4567, proto: TCP},
			str:       "1234-4567/tcp",
		},
		{
			in:        "1234-4567/udp",
			portRange: PortRange{start: 1234, end: 4567, proto: UDP},
			str:       "1234-4567/udp",
		},
		// Default protocol is tcp
		{
			in:        "1234-4567",
			portRange: PortRange{start: 1234, end: 4567, proto: TCP},
			str:       "1234-4567/tcp",
		},
		{
			in:        "1234/tcp",
			portRange: PortRange{start: 1234, end: 1234, proto: TCP},
			str:       "1234-1234/tcp",
		},
		{
			in:        "1234",
			portRange: PortRange{start: 1234, end: 1234, proto: TCP},
			str:       "1234-1234/tcp",
		},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParsePortRange(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.portRange {
				t.Errorf("expected port range %+v, got %+v", tc.portRange, got)
			}

			MustParsePortRange(tc.in) // should not panic

			// Check that ParsePortRange is a pure function.
			if got2, err := ParsePortRange(tc.in); err != nil {
				t.Fatal(err)
			} else if got2 != got {
				t.Errorf("ParsePortRange(%q) got 2 different results %#v, %#v", tc.in, got, got2)
			}

			// Check that ParsePortRange(port.String()) is the identity function.
			if got3, err := ParsePortRange(got.String()); err != nil {
				t.Fatal(err)
			} else if got3 != got {
				t.Errorf("ParsePortRange(%q) != ParsePortRange(ParsePortRange(%q).String()) Got %#v, want %#v", tc.in, tc.in, got3, got)
			}

			// Check String() output
			s := got.String()
			wants := tc.str
			if wants == "" {
				wants = tc.in
			}
			if s != wants {
				t.Errorf("ParsePortRange(%q).String() got %q, want %q", tc.in, s, wants)
			}

			js := `"` + tc.in + `"`
			var jsgot PortRange
			if err := json.Unmarshal([]byte(js), &jsgot); err != nil {
				t.Fatal(err)
			}
			if jsgot != got {
				t.Errorf("json.Unmarshal(%q) = %#v, want %#v", tc.in, jsgot, got)
			}
			jsb, err := json.Marshal(jsgot)
			if err != nil {
				t.Fatal(err)
			}
			jswant := `"` + wants + `"`
			jsback := string(jsb)
			if jsback != jswant {
				t.Errorf("json.Marshal(json.Unmarshal(%q)) = %q, want %q", tc.in, jsback, jswant)
			}

			// Check Range() output
			if r := got.Range(); r != tc.portRange {
				t.Errorf("ParsePortRange(%q).Range() = %+v, want %+v", tc.in, r, tc.portRange)
			}
		})

		negativeTests := []string{
			// Empty string
			"",
			// Whitespace-only string
			" ",
			// Negative start port
			"-1-1234",
			// Negative end port
			"1234--1",
			// Too large start port
			"65536-65537",
			// Too large end port
			"1234-65536",
			// Non-numeric start port
			"foo-1234",
			// Non-numeric end port
			"1234-bar",
			// Start port greater than end port
			"1234-1000",
			// Garbage port range
			"asd1234-5678/tcp",
		}

		for _, s := range negativeTests {
			t.Run(s, func(t *testing.T) {
				got, err := ParsePortRange(s)
				if err == nil {
					t.Errorf("ParsePortRange(%q), got port range %+v", s, got)
				}

				var jsgot PortRange
				js := []byte(`"` + s + `"`)
				if err := json.Unmarshal(js, &jsgot); err == nil {
					t.Errorf("json.Unmarshal(%q) = %#v", s, jsgot)
				}
			})
		}
	}
}
