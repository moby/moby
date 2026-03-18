package network_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

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
		assert.Check(t, p.IsZero())
		assert.Check(t, !p.IsValid())
		assert.Equal(t, p.String(), "invalid port")
		assert.Equal(t, p.Proto(), network.IPProtocol(""))
		assert.Equal(t, p.Num(), uint16(0))
		assert.Equal(t, p.Port(), "")
		assert.Equal(t, p.Range(), network.PortRange{})

		t.Run("Marshal Unmarshal", func(t *testing.T) {
			var p network.Port
			bytes, err := p.MarshalText()
			assert.NilError(t, err)
			assert.Check(t, len(bytes) == 0)

			err = p.UnmarshalText([]byte(""))
			assert.NilError(t, err)
			assert.Equal(t, p, network.Port{})
		})

		t.Run("JSON Marshal Unmarshal", func(t *testing.T) {
			var p network.Port
			bytes, err := json.Marshal(p)
			assert.NilError(t, err)
			assert.Equal(t, string(bytes), `""`)

			err = json.Unmarshal([]byte(`""`), &p)
			assert.NilError(t, err)
			assert.Equal(t, p, network.Port{})
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
				assert.Check(t, ok)
				assert.Equal(t, p.Num(), tc.num)
				assert.Equal(t, p.Port(), strconv.Itoa(int(tc.num)))
				assert.Equal(t, p.Proto(), tc.proto)
			})
		}

		t.Run("Normalize Protocol", func(t *testing.T) {
			pr1 := portFrom(1234, "tcp")
			pr2 := portFrom(1234, "TCP")
			pr3 := portFrom(1234, "tCp")
			assert.Equal(t, pr1, pr2)
			assert.Equal(t, pr2, pr3)
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
				assert.Check(t, !ok)
				assert.Check(t, p.IsZero())
				assert.Check(t, !p.IsValid())
				assert.Equal(t, p.String(), "invalid port")
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
				assert.NilError(t, err)
				assert.Equal(t, got, tc.port)

				network.MustParsePort(tc.in) // should not panic

				assert.Check(t, !got.IsZero())
				assert.Check(t, got.IsValid())

				// Check that ParsePort is a pure function.
				got2, err := network.ParsePort(tc.in)
				assert.NilError(t, err)
				assert.Equal(t, got2, got)

				// Check that ParsePort(port.String()) is the identity function.
				got3, err := network.ParsePort(got.String())
				assert.NilError(t, err)
				assert.Equal(t, got3, got)

				// Check String() output
				s := got.String()
				wants := tc.str
				if wants == "" {
					wants = tc.in
				}
				assert.Equal(t, s, wants)

				js := `"` + tc.in + `"`
				var jsgot network.Port
				err = json.Unmarshal([]byte(js), &jsgot)
				assert.NilError(t, err)
				assert.Equal(t, jsgot, got)

				jsb, err := json.Marshal(jsgot)
				assert.NilError(t, err)

				jswant := `"` + wants + `"`
				assert.Equal(t, string(jsb), jswant)

				// Check Range() output
				r := got.Range()
				assert.Equal(t, r, tc.portRange)
			})
		}

		t.Run("Normalize Protocol", func(t *testing.T) {
			p1 := network.MustParsePort("1234/tcp")
			p2 := network.MustParsePort("1234/TCP")
			p3 := network.MustParsePort("1234/tCp")
			assert.Equal(t, p1, p2)
			assert.Equal(t, p2, p3)
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
				assert.ErrorContains(t, err, "invalid port")
				assert.Check(t, got.IsZero())
				assert.Check(t, !got.IsValid())

				// Skip JSON unmarshalling test for empty string as that should succeed.
				// See test "Zero Value" above.
				if s == "" {
					return
				}

				var jsGot network.Port
				js := []byte(`"` + s + `"`)
				err = json.Unmarshal(js, &jsGot)
				assert.ErrorContains(t, err, "invalid port")
				assert.Equal(t, jsGot, network.Port{})
			})
		}
	})
}

func TestPortRange(t *testing.T) {
	t.Run("Zero Value", func(t *testing.T) {
		var pr network.PortRange
		assert.Check(t, pr.IsZero())
		assert.Check(t, !pr.IsValid())
		assert.Equal(t, pr.String(), "invalid port range")
		assert.Equal(t, pr.Start(), uint16(0))
		assert.Equal(t, pr.End(), uint16(0))
		assert.Equal(t, pr.Proto(), network.IPProtocol(""))
		assert.Equal(t, pr.Range(), pr)
		assert.Check(t, slices.Equal(slices.Collect(pr.All()), []network.Port{}))

		t.Run("Marshal Unmarshal", func(t *testing.T) {
			var pr network.PortRange
			bytes, err := pr.MarshalText()
			assert.NilError(t, err)
			assert.Check(t, len(bytes) == 0)

			err = pr.UnmarshalText([]byte(""))
			assert.NilError(t, err)
			assert.Equal(t, pr, network.PortRange{})
		})

		t.Run("JSON Marshal Unmarshal", func(t *testing.T) {
			var pr network.PortRange
			bytes, err := json.Marshal(pr)
			assert.NilError(t, err)
			assert.Equal(t, string(bytes), `""`)

			err = json.Unmarshal([]byte(`""`), &pr)
			assert.NilError(t, err)
			assert.Equal(t, pr, network.PortRange{})
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
				assert.Check(t, ok)
				assert.Equal(t, pr.Start(), tc.start)
				assert.Equal(t, pr.End(), tc.end)
				assert.Equal(t, pr.Proto(), tc.proto)
			})
		}

		t.Run("Normalize Protocol", func(t *testing.T) {
			pr1, _ := network.PortRangeFrom(1234, 5678, "tcp")
			pr2, _ := network.PortRangeFrom(1234, 5678, "TCP")
			pr3, _ := network.PortRangeFrom(1234, 5678, "tCp")
			assert.Equal(t, pr1, pr2)
			assert.Equal(t, pr2, pr3)
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
				assert.Check(t, !ok)
				assert.Check(t, pr.IsZero())
				assert.Check(t, !pr.IsValid())
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
				assert.NilError(t, err)
				assert.Equal(t, got, tc.portRange)
				assert.Check(t, !got.IsZero())
				assert.Check(t, got.IsValid())

				network.MustParsePortRange(tc.in) // should not panic

				// Check that ParsePortRange is a pure function.
				got2, err := network.ParsePortRange(tc.in)
				assert.NilError(t, err)
				assert.Equal(t, got2, got)

				// Check that ParsePortRange(port.String()) is the identity function.
				got3, err := network.ParsePortRange(got.String())
				assert.NilError(t, err)
				assert.Equal(t, got3, got)

				// Check String() output
				s := got.String()
				wants := tc.str
				if wants == "" {
					wants = tc.in
				}
				assert.Equal(t, s, wants)

				js := `"` + tc.in + `"`
				var jsgot network.PortRange
				err = json.Unmarshal([]byte(js), &jsgot)
				assert.NilError(t, err)
				assert.Equal(t, jsgot, got)

				jsb, err := json.Marshal(jsgot)
				assert.NilError(t, err)
				jswant := `"` + wants + `"`
				assert.Equal(t, string(jsb), jswant)

				// Check Range() output
				r := got.Range()
				assert.Equal(t, r, tc.portRange)
			})

			t.Run("Normalize Protocol", func(t *testing.T) {
				pr1 := network.MustParsePortRange("1234-5678/tcp")
				pr2 := network.MustParsePortRange("1234-5678/TCP")
				pr3 := network.MustParsePortRange("1234-5678/tCp")
				assert.Equal(t, pr1, pr2)
				assert.Equal(t, pr2, pr3)
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
					assert.Check(t, err != nil)
					assert.Check(t, got.IsZero())
					assert.Check(t, !got.IsValid())

					// Skip JSON unmarshalling test for empty string as that should succeed.
					// See test "Zero Value" above.
					if s == "" {
						return
					}

					var jsgot network.PortRange
					js := []byte(`"` + s + `"`)
					err = json.Unmarshal(js, &jsgot)
					assert.Check(t, err != nil)
					assert.Equal(t, jsgot, network.PortRange{})
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
