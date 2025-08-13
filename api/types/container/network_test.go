package container

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type TestRanger interface {
	Range() PortRange
}

var _ TestRanger = Port{}
var _ TestRanger = PortRange{}

func TestPort(t *testing.T) {
	t.Run("Zero Value", func(t *testing.T) {
		var p Port
		if !p.IsZero() {
			t.Errorf("Port{}.IsZero() = false, want true")
		}
		if p.IsValid() {
			t.Errorf("Port{}.IsValid() = true, want false")
		}
		if p.String() != "invalid port" {
			t.Errorf("Port{}.String() = %q, want %q", p.String(), "invalid port")
		}

		t.Run("Marshal Unmarshal", func(t *testing.T) {
			var p Port
			bytes, err := p.MarshalText()
			if err != nil {
				t.Errorf("Port{}.MarshalText() error: %v", err)
			}
			if len(bytes) != 0 {
				t.Errorf("Port{}.MarshalText() = %q, want empty string", string(bytes))
			}

			err = p.UnmarshalText([]byte(""))
			if err != nil {
				t.Errorf("Port{}.UnmarshalText(\"\") error: %v", err)
			}
			if p != (Port{}) {
				t.Errorf("Port{}.UnmarshalText(\"\") = %#v, want %#v", p, Port{})
			}
		})

		t.Run("JSON Marshal Unmarshal", func(t *testing.T) {
			var p Port
			bytes, err := json.Marshal(p)
			if err != nil {
				t.Errorf("json.Marshal(Port{}) error: %v", err)
			}
			if string(bytes) != `""` {
				t.Errorf("json.Marshal(Port{}) = %q, want %q", string(bytes), `""`)
			}

			err = json.Unmarshal([]byte(`""`), &p)
			if err != nil {
				t.Errorf("json.Unmarshal(`\"\"`) error: %v", err)
			}
			if p != (Port{}) {
				t.Errorf("json.Unmarshal(`\"\"`) = %#v, want %#v", p, Port{})
			}
		})
	})

	t.Run("PortFrom", func(t *testing.T) {
		tests := []struct {
			num   uint16
			proto PortProto
		}{
			{0, TCP},
			{80, TCP},
			{8080, TCP},
			{65535, TCP},
			{80, UDP},
			{8080, SCTP},
		}

		for _, tc := range tests {
			t.Run(fmt.Sprintf("%d_%s", tc.num, tc.proto), func(t *testing.T) {
				p := PortFrom(tc.num, tc.proto)
				if p.Num() != tc.num {
					t.Errorf("PortFrom(%d, %q).Num() = %d, want %d", tc.num, tc.proto, p.Num(), tc.num)
				}
				if p.Proto() != tc.proto {
					t.Errorf("PortFrom(%d, %q).Proto() = %q, want %q", tc.num, tc.proto, p.Proto(), tc.proto)
				}
			})
		}

		t.Run("Normalize Protocol", func(t *testing.T) {
			pr1 := PortFrom(1234, "tcp")
			pr2 := PortFrom(1234, "TCP")
			pr3 := PortFrom(1234, "tCp")
			if pr1 != pr2 || pr2 != pr3 {
				t.Errorf("PortFrom protocol normalization failed: %q, %q, %q", pr1, pr2, pr3)
			}
		})

		negativeTests := []struct {
			num   uint16
			proto PortProto
		}{
			{0, ""},
			{80, ""},
		}
		for _, tc := range negativeTests {
			t.Run(fmt.Sprintf("%d_%s", tc.num, tc.proto), func(t *testing.T) {
				p := PortFrom(tc.num, tc.proto)
				if !p.IsZero() {
					t.Errorf("PortFrom(%d, %q).IsZero() = false, want true", tc.num, tc.proto)
				}
				if p.IsValid() {
					t.Errorf("PortFrom(%d, %q).IsValid() = true, want false", tc.num, tc.proto)
				}
				if p.String() != "invalid port" {
					t.Errorf("PortFrom(%d, %q).String() = %q, want %q", tc.num, tc.proto, p.String(), "invalid port")
				}
			})
		}
	})

	t.Run("ParsePort", func(t *testing.T) {
		tests := []struct {
			in        string
			port      Port      // output of ParsePort()
			str       string    // output of String().
			portRange PortRange // output of Range()
		}{
			// Zero port
			{
				in:        "0/tcp",
				port:      PortFrom(0, TCP),
				str:       "0/tcp",
				portRange: portRangeFrom(0, 0, TCP),
			},
			// Max valid port
			{
				in:        "65535/tcp",
				port:      PortFrom(65535, TCP),
				str:       "65535/tcp",
				portRange: portRangeFrom(65535, 65535, TCP),
			},
			// Simple valid ports
			{
				in:        "1234/tcp",
				port:      PortFrom(1234, TCP),
				str:       "1234/tcp",
				portRange: portRangeFrom(1234, 1234, TCP),
			},
			{
				in:        "1234/udp",
				port:      PortFrom(1234, UDP),
				str:       "1234/udp",
				portRange: portRangeFrom(1234, 1234, UDP),
			},
			{
				in:        "1234/sctp",
				port:      PortFrom(1234, SCTP),
				str:       "1234/sctp",
				portRange: portRangeFrom(1234, 1234, SCTP),
			},
			// Default protocol is tcp
			{
				in:        "1234",
				port:      PortFrom(1234, TCP),
				str:       "1234/tcp",
				portRange: portRangeFrom(1234, 1234, TCP),
			},
			// Default protocol is tcp
			{
				in:        "1234/",
				port:      PortFrom(1234, TCP),
				str:       "1234/tcp",
				portRange: portRangeFrom(1234, 1234, TCP),
			},
			{
				in:        "1234/tcp:ipv6only",
				port:      PortFrom(1234, "tcp:ipv6only"),
				str:       "1234/tcp:ipv6only",
				portRange: portRangeFrom(1234, 1234, "tcp:ipv6only"),
			},
		}

		for _, tc := range tests {
			t.Run(strings.ReplaceAll(tc.in, "/", "_"), func(t *testing.T) {
				got, err := ParsePort(tc.in)
				if err != nil {
					t.Fatal(err)
				}
				if got != tc.port {
					t.Errorf("expected port %+v, got %+v", tc.port, got)
				}

				MustParsePort(tc.in) // should not panic

				if got.IsZero() {
					t.Errorf("ParsePort(%q).IsZero() = true, want false", tc.in)
				}

				if !got.IsValid() {
					t.Errorf("ParsePort(%q).IsValid() = false, want true", tc.in)
				}

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

		t.Run("Normalize Protocol", func(t *testing.T) {
			p1 := MustParsePort("1234/tcp")
			p2 := MustParsePort("1234/TCP")
			p3 := MustParsePort("1234/tCp")
			if p1 != p2 || p2 != p3 {
				t.Errorf("Port protocol normalization failed: %q, %q, %q", p1, p2, p3)
			}
		})

		negativeTests := []string{
			// Empty string
			"",
			// Whitespace-only string
			" ",
			// No port number
			"/",
			// No port number (protocol only)
			"/tcp",
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
			t.Run(strings.ReplaceAll(s, "/", "_"), func(t *testing.T) {
				got, err := ParsePort(s)
				if err == nil {
					t.Errorf("ParsePort(%q), got port %+v", s, got)
				}

				if !got.IsZero() {
					t.Errorf("ParsePort(%q).IsZero() = false, want true", s)
				}

				if got.IsValid() {
					t.Errorf("ParsePort(%q).IsValid() = true, want false", s)
				}

				// Skip JSON unmarshalling test for empty string as that should succeed.
				// See TestUnmarshalEmptyPort below.
				if s == "" {
					return
				}

				var jsgot Port
				js := []byte(`"` + s + `"`)
				if err := json.Unmarshal(js, &jsgot); err == nil {
					t.Errorf("json.Unmarshal(%q) = %#v", s, jsgot)
				}
			})
		}
	})
}

func TestPortRange(t *testing.T) {
	t.Run("Zero Value", func(t *testing.T) {
		var pr PortRange
		if !pr.IsZero() {
			t.Errorf("PortRange{}.IsZero() = false, want true")
		}
		if pr.IsValid() {
			t.Errorf("PortRange{}.IsValid() = true, want false")
		}
		if pr.String() != "invalid port range" {
			t.Errorf("PortRange{}.String() = %q, want %q", pr.String(), "invalid port range")
		}

		t.Run("Marshal Unmarshal", func(t *testing.T) {
			var pr PortRange
			bytes, err := pr.MarshalText()
			if err != nil {
				t.Errorf("PortRange{}.MarshalText() error: %v", err)
			}
			if len(bytes) != 0 {
				t.Errorf("PortRange{}.MarshalText() = %q, want empty string", string(bytes))
			}

			err = pr.UnmarshalText([]byte(""))
			if err != nil {
				t.Errorf("PortRange{}.UnmarshalText(\"\") error: %v", err)
			}
			if pr != (PortRange{}) {
				t.Errorf("PortRange{}.UnmarshalText(\"\") = %#v, want %#v", pr, PortRange{})
			}
		})

		t.Run("JSON Marshal Unmarshal", func(t *testing.T) {
			var pr PortRange
			bytes, err := json.Marshal(pr)
			if err != nil {
				t.Errorf("json.Marshal(PortRange{}) error: %v", err)
			}
			if string(bytes) != `""` {
				t.Errorf("json.Marshal(PortRange{}) = %q, want %q", string(bytes), `""`)
			}

			err = json.Unmarshal([]byte(`""`), &pr)
			if err != nil {
				t.Errorf("json.Unmarshal(`\"\"`) error: %v", err)
			}
			if pr != (PortRange{}) {
				t.Errorf("json.Unmarshal(`\"\"`) = %#v, want %#v", pr, PortRange{})
			}
		})
	})

	t.Run("PortRangeFrom", func(t *testing.T) {
		tests := []struct {
			start uint16
			end   uint16
			proto PortProto
		}{
			{0, 0, TCP},
			{0, 1234, TCP},
			{80, 80, TCP},
			{80, 8080, TCP},
			{1234, 65535, TCP},
			{80, 80, UDP},
			{80, 8080, SCTP},
		}

		for _, tc := range tests {
			t.Run(fmt.Sprintf("%d_%d_%s", tc.start, tc.end, tc.proto), func(t *testing.T) {
				pr, err := PortRangeFrom(tc.start, tc.end, tc.proto)
				if err != nil {
					t.Fatalf("PortRangeFrom(%d, %d, %q) error: %v", tc.start, tc.end, tc.proto, err)
				}
				if pr.Start() != tc.start {
					t.Errorf("PortRangeFrom(%d, %d, %q).Start() = %d, want %d", tc.start, tc.end, tc.proto, pr.Start(), tc.start)
				}
				if pr.End() != tc.end {
					t.Errorf("PortRangeFrom(%d, %d, %q).End() = %d, want %d", tc.start, tc.end, tc.proto, pr.End(), tc.end)
				}
				if pr.Proto() != tc.proto {
					t.Errorf("PortRangeFrom(%d, %d, %q).Proto() = %q, want %q", tc.start, tc.end, tc.proto, pr.Proto(), tc.proto)
				}
			})
		}

		t.Run("Normalize Protocol", func(t *testing.T) {
			pr1, _ := PortRangeFrom(1234, 5678, "tcp")
			pr2, _ := PortRangeFrom(1234, 5678, "TCP")
			pr3, _ := PortRangeFrom(1234, 5678, "tCp")
			if pr1 != pr2 || pr2 != pr3 {
				t.Errorf("PortRangeFrom protocol normalization failed: %q, %q, %q", pr1, pr2, pr3)
			}
		})

		negativeTests := []struct {
			start    uint16
			end      uint16
			proto    PortProto
			expError bool
		}{
			{1234, 80, TCP, true}, // end < start
			{0, 0, "", false},     // empty protocol
		}
		for _, tc := range negativeTests {
			t.Run(fmt.Sprintf("%d_%d_%s", tc.start, tc.end, tc.proto), func(t *testing.T) {
				pr, err := PortRangeFrom(tc.start, tc.end, tc.proto)
				if tc.expError && err == nil {
					t.Errorf("PortRangeFrom(%d, %d, %q) = %+v", tc.start, tc.end, tc.proto, pr)
				}
				if !tc.expError && err != nil {
					t.Errorf("PortRangeFrom(%d, %d, %q) error: %v", tc.start, tc.end, tc.proto, err)
				}
				if !pr.IsZero() {
					t.Errorf("PortRangeFrom(%d, %d, %q).IsZero() = false, want true", tc.start, tc.end, tc.proto)
				}
				if pr.IsValid() {
					t.Errorf("PortRangeFrom(%d, %d, %q).IsValid() = true, want false", tc.start, tc.end, tc.proto)
				}
			})
		}
	})

	t.Run("ParsePortRange", func(t *testing.T) {
		tests := []struct {
			in        string
			portRange PortRange // output of ParsePortRange() and Range()
			str       string    // output of String(). If "", use in.

		}{
			// Zero port
			{
				in:        "0-1234/tcp",
				portRange: portRangeFrom(0, 1234, TCP),
				str:       "0-1234/tcp",
			},
			// Max valid port
			{
				in:        "1234-65535/tcp",
				portRange: portRangeFrom(1234, 65535, TCP),
				str:       "1234-65535/tcp",
			},
			// Simple valid ports
			{
				in:        "1234-4567/tcp",
				portRange: portRangeFrom(1234, 4567, TCP),
				str:       "1234-4567/tcp",
			},
			{
				in:        "1234-4567/udp",
				portRange: portRangeFrom(1234, 4567, UDP),
				str:       "1234-4567/udp",
			},
			// Default protocol is tcp
			{
				in:        "1234-4567",
				portRange: portRangeFrom(1234, 4567, TCP),
				str:       "1234-4567/tcp",
			},
			// Default protocol is tcp
			{
				in:        "1234-4567/",
				portRange: portRangeFrom(1234, 4567, TCP),
				str:       "1234-4567/tcp",
			},
			{
				in:        "1234/tcp",
				portRange: portRangeFrom(1234, 1234, TCP),
				str:       "1234-1234/tcp",
			},
			{
				in:        "1234",
				portRange: portRangeFrom(1234, 1234, TCP),
				str:       "1234-1234/tcp",
			},
			{
				in:        "1234-5678/tcp:ipv6only",
				portRange: portRangeFrom(1234, 5678, "tcp:ipv6only"),
				str:       "1234-5678/tcp:ipv6only",
			},
		}

		for _, tc := range tests {
			t.Run(strings.ReplaceAll(tc.in, "/", "_"), func(t *testing.T) {
				got, err := ParsePortRange(tc.in)
				if err != nil {
					t.Fatal(err)
				}
				if got != tc.portRange {
					t.Errorf("expected port range %+v, got %+v", tc.portRange, got)
				}

				if got.IsZero() {
					t.Errorf("ParsePortRange(%q).IsZero() = true, want false", tc.in)
				}

				if !got.IsValid() {
					t.Errorf("ParsePortRange(%q).IsValid() = false, want true", tc.in)
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

			t.Run("Normalize Protocol", func(t *testing.T) {
				pr1 := MustParsePortRange("1234-5678/tcp")
				pr2 := MustParsePortRange("1234-5678/TCP")
				pr3 := MustParsePortRange("1234-5678/tCp")
				if pr1 != pr2 || pr2 != pr3 {
					t.Errorf("PortRange protocol normalization failed: %q, %q, %q", pr1, pr2, pr3)
				}
			})

			negativeTests := []string{
				// Empty string
				"",
				// Whitespace-only string
				" ",
				// No port number
				"/",
				// No port number (protocol only)
				"/tcp",
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
				t.Run(strings.ReplaceAll(s, "/", "_"), func(t *testing.T) {
					got, err := ParsePortRange(s)
					if err == nil {
						t.Errorf("ParsePortRange(%q), got port range %+v", s, got)
					}

					if !got.IsZero() {
						t.Errorf("ParsePortRange(%q).IsZero() = false, want true", s)
					}

					if got.IsValid() {
						t.Errorf("ParsePortRange(%q).IsValid() = true, want false", s)
					}

					// Skip JSON unmarshalling test for empty string as that should succeed.
					// See TestUnmarshalEmptyPortRange below.
					if s == "" {
						return
					}

					var jsgot PortRange
					js := []byte(`"` + s + `"`)
					if err := json.Unmarshal(js, &jsgot); err == nil {
						t.Errorf("json.Unmarshal(%q) = %#v", s, jsgot)
					}
				})
			}
		}
	})
}

func portRangeFrom(start, end uint16, proto PortProto) PortRange {
	pr, err := PortRangeFrom(start, end, proto)
	if err != nil {
		panic(err)
	}
	return pr
}
