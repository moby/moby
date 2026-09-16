package network_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/network"
)

type TestRanger interface {
	Range() network.PortRange
}

var (
	_ TestRanger = network.Port{}
	_ TestRanger = network.PortRange{}
)

func TestPort(t *testing.T) {
	t.Run("Zero Value", func(t *testing.T) {
		var p network.Port
		assertEqual(t, p.IsZero(), true)
		assertEqual(t, p.IsValid(), false)
		assertEqual(t, p.String(), "invalid port")
		assertEqual(t, p.Proto(), network.IPProtocol(""))
		assertEqual(t, p.Num(), uint16(0))
		assertEqual(t, p.Port(), "")
		assertEqual(t, p.Range(), network.PortRange{})

		t.Run("Marshal Unmarshal", func(t *testing.T) {
			var p network.Port
			bytes, err := p.MarshalText()
			assertNoError(t, err)
			assertEqual(t, len(bytes), 0)

			err = p.UnmarshalText([]byte(""))
			assertNoError(t, err)
			assertEqual(t, p, network.Port{})
		})

		t.Run("JSON Marshal Unmarshal", func(t *testing.T) {
			var p network.Port
			bytes, err := json.Marshal(p)
			assertNoError(t, err)
			assertEqual(t, string(bytes), `""`)

			err = json.Unmarshal([]byte(`""`), &p)
			assertNoError(t, err)
			assertEqual(t, p, network.Port{})
		})
	})

	t.Run("PortFrom", func(t *testing.T) {
		tests := []struct {
			num   uint16
			proto network.IPProtocol
		}{
			{proto: network.TCP},
			{num: 80, proto: network.TCP},
			{num: 8080, proto: network.TCP},
			{num: 65535, proto: network.TCP},
			{num: 80, proto: network.UDP},
			{num: 8080, proto: network.SCTP},
		}

		for _, tc := range tests {
			t.Run(fmt.Sprintf("%d_%s", tc.num, tc.proto), func(t *testing.T) {
				p, ok := network.PortFrom(tc.num, tc.proto)
				assertEqual(t, ok, true)
				assertEqual(t, p.Num(), tc.num)
				assertEqual(t, p.Port(), strconv.Itoa(int(tc.num)))
				assertEqual(t, p.Proto(), tc.proto)
			})
		}

		t.Run("Normalize Protocol", func(t *testing.T) {
			pr1 := portFrom(1234, "tcp")
			pr2 := portFrom(1234, "TCP")
			pr3 := portFrom(1234, "tCp")
			assertEqual(t, pr1, pr2)
			assertEqual(t, pr2, pr3)
		})

		negativeTests := []struct {
			num   uint16
			proto network.IPProtocol
		}{
			{num: 0, proto: ""},
			{num: 80, proto: ""},
		}
		for _, tc := range negativeTests {
			t.Run(fmt.Sprintf("%d_%s", tc.num, tc.proto), func(t *testing.T) {
				p, ok := network.PortFrom(tc.num, tc.proto)
				assertEqual(t, ok, false)
				assertEqual(t, p.IsZero(), true)
				assertEqual(t, p.IsValid(), false)
				assertEqual(t, p.String(), "invalid port")
			})
		}
	})

	t.Run("ParsePort", func(t *testing.T) {
		tests := []struct {
			in        string
			port      network.Port      // output of ParsePort()
			str       string            // output of String().
			portRange network.PortRange // output of Range()
		}{
			// Zero network.Port
			{
				in:        "0/tcp",
				port:      portFrom(0, network.TCP),
				str:       "0/tcp",
				portRange: portRangeFrom(0, 0, network.TCP),
			},
			// Max valid network.Port
			{
				in:        "65535/tcp",
				port:      portFrom(65535, network.TCP),
				str:       "65535/tcp",
				portRange: portRangeFrom(65535, 65535, network.TCP),
			},
			// Simple valid network.Ports
			{
				in:        "1234/tcp",
				port:      portFrom(1234, network.TCP),
				str:       "1234/tcp",
				portRange: portRangeFrom(1234, 1234, network.TCP),
			},
			{
				in:        "1234/udp",
				port:      portFrom(1234, network.UDP),
				str:       "1234/udp",
				portRange: portRangeFrom(1234, 1234, network.UDP),
			},
			{
				in:        "1234/sctp",
				port:      portFrom(1234, network.SCTP),
				str:       "1234/sctp",
				portRange: portRangeFrom(1234, 1234, network.SCTP),
			},
			// Default protocol is tcp
			{
				in:        "1234",
				port:      portFrom(1234, network.TCP),
				str:       "1234/tcp",
				portRange: portRangeFrom(1234, 1234, network.TCP),
			},
			// Default protocol is tcp
			{
				in:        "1234/",
				port:      portFrom(1234, network.TCP),
				str:       "1234/tcp",
				portRange: portRangeFrom(1234, 1234, network.TCP),
			},
			{
				in:        "1234/tcp:ipv6only",
				port:      portFrom(1234, "tcp:ipv6only"),
				str:       "1234/tcp:ipv6only",
				portRange: portRangeFrom(1234, 1234, "tcp:ipv6only"),
			},
		}

		for _, tc := range tests {
			t.Run(strings.ReplaceAll(tc.in, "/", "_"), func(t *testing.T) {
				got, err := network.ParsePort(tc.in)
				assertNoError(t, err)
				assertEqual(t, got, tc.port)

				network.MustParsePort(tc.in) // should not panic

				assertEqual(t, got.IsZero(), false)
				assertEqual(t, got.IsValid(), true)

				// Check that ParsePort is a pure function.
				got2, err := network.ParsePort(tc.in)
				assertNoError(t, err)
				assertEqual(t, got2, got)

				// Check that ParsePort(port.String()) is the identity function.
				got3, err := network.ParsePort(got.String())
				assertNoError(t, err)
				assertEqual(t, got3, got)

				// Check String() output
				s := got.String()
				wants := tc.str
				if wants == "" {
					wants = tc.in
				}
				assertEqual(t, s, wants)

				js := `"` + tc.in + `"`
				var jsgot network.Port
				err = json.Unmarshal([]byte(js), &jsgot)
				assertNoError(t, err)
				assertEqual(t, jsgot, got)

				jsb, err := json.Marshal(jsgot)
				assertNoError(t, err)

				jswant := `"` + wants + `"`
				assertEqual(t, string(jsb), jswant)

				// Check Range() output
				r := got.Range()
				assertEqual(t, r, tc.portRange)
			})
		}

		t.Run("Normalize Protocol", func(t *testing.T) {
			p1 := network.MustParsePort("1234/tcp")
			p2 := network.MustParsePort("1234/TCP")
			p3 := network.MustParsePort("1234/tCp")
			assertEqual(t, p1, p2)
			assertEqual(t, p2, p3)
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
				got, err := network.ParsePort(s)
				assertErrorContains(t, err, "invalid port")
				assertEqual(t, got.IsZero(), true)
				assertEqual(t, got.IsValid(), false)

				// Skip JSON unmarshalling test for empty string as that should succeed.
				// See test "Zero Value" above.
				if s == "" {
					return
				}

				var jsGot network.Port
				js := []byte(`"` + s + `"`)
				err = json.Unmarshal(js, &jsGot)
				assertErrorContains(t, err, "invalid port")
				assertEqual(t, jsGot, network.Port{})
			})
		}
	})
}

func TestPortRange(t *testing.T) {
	t.Run("Zero Value", func(t *testing.T) {
		var pr network.PortRange
		assertEqual(t, pr.IsZero(), true)
		assertEqual(t, pr.IsValid(), false)
		assertEqual(t, pr.String(), "invalid port range")
		assertEqual(t, pr.Start(), uint16(0))
		assertEqual(t, pr.End(), uint16(0))
		assertEqual(t, pr.Proto(), network.IPProtocol(""))
		assertEqual(t, pr.Range(), pr)
		var ephemeralPort network.Port
		if got := slices.Collect(pr.All()); !slices.Equal(got, []network.Port{ephemeralPort}) {
			t.Errorf("PortRange.All() = %#v, want %#v", got, []network.Port{ephemeralPort})
		}

		t.Run("Marshal Unmarshal", func(t *testing.T) {
			var pr network.PortRange
			bytes, err := pr.MarshalText()
			assertNoError(t, err)
			assertEqual(t, len(bytes), 0)

			err = pr.UnmarshalText([]byte(""))
			assertNoError(t, err)
			assertEqual(t, pr, network.PortRange{})
		})

		t.Run("JSON Marshal Unmarshal", func(t *testing.T) {
			var pr network.PortRange
			bytes, err := json.Marshal(pr)
			assertNoError(t, err)
			assertEqual(t, string(bytes), `""`)

			err = json.Unmarshal([]byte(`""`), &pr)
			assertNoError(t, err)
			assertEqual(t, pr, network.PortRange{})
		})
	})

	t.Run("PortRangeFrom", func(t *testing.T) {
		tests := []struct {
			start uint16
			end   uint16
			proto network.IPProtocol
		}{
			{start: 0, end: 0, proto: network.TCP},
			{start: 0, end: 1234, proto: network.TCP},
			{start: 80, end: 80, proto: network.TCP},
			{start: 80, end: 8080, proto: network.TCP},
			{start: 1234, end: 65535, proto: network.TCP},
			{start: 80, end: 80, proto: network.UDP},
			{start: 80, end: 8080, proto: network.SCTP},
		}

		for _, tc := range tests {
			t.Run(fmt.Sprintf("%d_%d_%s", tc.start, tc.end, tc.proto), func(t *testing.T) {
				pr, ok := network.PortRangeFrom(tc.start, tc.end, tc.proto)
				assertEqual(t, ok, true)
				assertEqual(t, pr.Start(), tc.start)
				assertEqual(t, pr.End(), tc.end)
				assertEqual(t, pr.Proto(), tc.proto)
			})
		}

		t.Run("Normalize Protocol", func(t *testing.T) {
			pr1, _ := network.PortRangeFrom(1234, 5678, "tcp")
			pr2, _ := network.PortRangeFrom(1234, 5678, "TCP")
			pr3, _ := network.PortRangeFrom(1234, 5678, "tCp")
			assertEqual(t, pr1, pr2)
			assertEqual(t, pr2, pr3)
		})

		negativeTests := []struct {
			start uint16
			end   uint16
			proto network.IPProtocol
		}{
			{start: 1234, end: 80, proto: network.TCP}, // end < start
			{}, // empty protocol
		}
		for _, tc := range negativeTests {
			t.Run(fmt.Sprintf("%d_%d_%s", tc.start, tc.end, tc.proto), func(t *testing.T) {
				pr, ok := network.PortRangeFrom(tc.start, tc.end, tc.proto)
				assertEqual(t, ok, false)
				assertEqual(t, pr.IsZero(), true)
				assertEqual(t, pr.IsValid(), false)
			})
		}
	})

	t.Run("ParsePortRange", func(t *testing.T) {
		tests := []struct {
			in        string
			portRange network.PortRange // output of network.ParsePortRange() and Range()
			str       string            // output of String(). If "", use in.
		}{
			// Zero port
			{
				in:        "0-1234/tcp",
				portRange: portRangeFrom(0, 1234, network.TCP),
				str:       "0-1234/tcp",
			},
			// Max valid port
			{
				in:        "1234-65535/tcp",
				portRange: portRangeFrom(1234, 65535, network.TCP),
				str:       "1234-65535/tcp",
			},
			// Simple valid ports
			{
				in:        "1234-4567/tcp",
				portRange: portRangeFrom(1234, 4567, network.TCP),
				str:       "1234-4567/tcp",
			},
			{
				in:        "1234-4567/udp",
				portRange: portRangeFrom(1234, 4567, network.UDP),
				str:       "1234-4567/udp",
			},
			// Default protocol is tcp
			{
				in:        "1234-4567",
				portRange: portRangeFrom(1234, 4567, network.TCP),
				str:       "1234-4567/tcp",
			},
			// Default protocol is tcp
			{
				in:        "1234-4567/",
				portRange: portRangeFrom(1234, 4567, network.TCP),
				str:       "1234-4567/tcp",
			},
			{
				in:        "1234/tcp",
				portRange: portRangeFrom(1234, 1234, network.TCP),
				str:       "1234/tcp",
			},
			{
				in:        "1234",
				portRange: portRangeFrom(1234, 1234, network.TCP),
				str:       "1234/tcp",
			},
			{
				in:        "1234-5678/tcp:ipv6only",
				portRange: portRangeFrom(1234, 5678, "tcp:ipv6only"),
				str:       "1234-5678/tcp:ipv6only",
			},
		}

		for _, tc := range tests {
			t.Run(strings.ReplaceAll(tc.in, "/", "_"), func(t *testing.T) {
				got, err := network.ParsePortRange(tc.in)
				assertNoError(t, err)
				assertEqual(t, got, tc.portRange)
				assertEqual(t, got.IsZero(), false)
				assertEqual(t, got.IsValid(), true)

				network.MustParsePortRange(tc.in) // should not panic

				// Check that ParsePortRange is a pure function.
				got2, err := network.ParsePortRange(tc.in)
				assertNoError(t, err)
				assertEqual(t, got2, got)

				// Check that ParsePortRange(port.String()) is the identity function.
				got3, err := network.ParsePortRange(got.String())
				assertNoError(t, err)
				assertEqual(t, got3, got)

				// Check String() output
				s := got.String()
				wants := tc.str
				if wants == "" {
					wants = tc.in
				}
				assertEqual(t, s, wants)

				js := `"` + tc.in + `"`
				var jsgot network.PortRange
				err = json.Unmarshal([]byte(js), &jsgot)
				assertNoError(t, err)
				assertEqual(t, jsgot, got)

				jsb, err := json.Marshal(jsgot)
				assertNoError(t, err)
				jswant := `"` + wants + `"`
				assertEqual(t, string(jsb), jswant)

				// Check Range() output
				r := got.Range()
				assertEqual(t, r, tc.portRange)
			})

			t.Run("Normalize Protocol", func(t *testing.T) {
				pr1 := network.MustParsePortRange("1234-5678/tcp")
				pr2 := network.MustParsePortRange("1234-5678/TCP")
				pr3 := network.MustParsePortRange("1234-5678/tCp")
				assertEqual(t, pr1, pr2)
				assertEqual(t, pr2, pr3)
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
					got, err := network.ParsePortRange(s)
					if err == nil {
						t.Error("expected error")
					}
					assertEqual(t, got.IsZero(), true)
					assertEqual(t, got.IsValid(), false)

					// Skip JSON unmarshalling test for empty string as that should succeed.
					// See test "Zero Value" above.
					if s == "" {
						return
					}

					var jsgot network.PortRange
					js := []byte(`"` + s + `"`)
					err = json.Unmarshal(js, &jsgot)
					if err == nil {
						t.Error("expected error")
					}
					assertEqual(t, jsgot, network.PortRange{})
				})
			}
		}
	})

	t.Run("PortRange All()", func(t *testing.T) {
		tests := []struct {
			in   string
			want []network.Port
		}{
			{
				in:   "1000-1000/tcp",
				want: []network.Port{portFrom(1000, network.TCP)},
			},
			{
				in:   "1000-1002/tcp",
				want: []network.Port{portFrom(1000, network.TCP), portFrom(1001, network.TCP), portFrom(1002, network.TCP)},
			},
			{
				in:   "0-0/tcp", // TODO(thaJeztah): should this result in "zero-value range" and an empty list?
				want: []network.Port{portFrom(0, network.TCP)},
			},
			{
				in:   "65535-65535/tcp",
				want: []network.Port{portFrom(65535, network.TCP)},
			},
			{
				in:   "65530-65535/tcp",
				want: []network.Port{portFrom(65530, network.TCP), portFrom(65531, network.TCP), portFrom(65532, network.TCP), portFrom(65533, network.TCP), portFrom(65534, network.TCP), portFrom(65535, network.TCP)},
			},
		}

		for _, tc := range tests {
			pr := network.MustParsePortRange(tc.in)
			ports := slices.Collect(pr.All())
			if !slices.Equal(ports, tc.want) {
				t.Errorf("PortRange.All() = %#v, want %#v", ports, tc.want)
			}
		}

		t.Run("All() stop early", func(t *testing.T) {
			want := []network.Port{portFrom(1000, network.TCP), portFrom(1001, network.TCP)}
			pr := network.MustParsePortRange("1000-2000/tcp")
			var ports []network.Port
			for p := range pr.All() {
				ports = append(ports, p)
				if len(ports) == 2 {
					break
				}
			}
			if !slices.Equal(ports, want) {
				t.Errorf("PortRange.All() = %#v, want %#v", ports, want)
			}
		})
	})
}

func BenchmarkPortRangeAll(b *testing.B) {
	b.Run("Single Port", func(b *testing.B) {
		pr := network.MustParsePortRange("1234/tcp")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var sink int64
			for p := range pr.All() {
				sink += int64(p.Num()) // prevent compiler optimization
			}
			if sink < 0 {
				b.Fatal("unreachable")
			}
		}
	})

	b.Run("Range", func(b *testing.B) {
		pr := network.MustParsePortRange("0-65535/tcp")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var sink int64
			for p := range pr.All() {
				sink += int64(p.Num()) // prevent compiler optimization
			}
			if sink < 0 {
				b.Fatal("unreachable")
			}
		}
	})
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	} else if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want error containing %q", err, want)
	}
}

func assertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func portFrom(num uint16, proto network.IPProtocol) network.Port {
	p, ok := network.PortFrom(num, proto)
	if !ok {
		panic("invalid port")
	}
	return p
}

func portRangeFrom(start, end uint16, proto network.IPProtocol) network.PortRange {
	pr, ok := network.PortRangeFrom(start, end, proto)
	if !ok {
		panic("invalid port range")
	}
	return pr
}
