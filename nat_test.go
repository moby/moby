package nat

import (
	"reflect"
	"testing"
)

func TestParsePort(t *testing.T) {
	tests := []struct {
		doc     string
		input   string
		expPort int
		expErr  string
	}{
		{
			doc:     "invalid value",
			input:   "asdf",
			expPort: 0,
			expErr:  `invalid port 'asdf': invalid syntax`,
		},
		{
			doc:     "invalid value with number",
			input:   "1asdf",
			expPort: 0,
			expErr:  `invalid port '1asdf': invalid syntax`,
		},
		{
			doc:     "empty value",
			input:   "",
			expPort: 0,
		},
		{
			doc:     "zero value",
			input:   "0",
			expPort: 0,
		},
		{
			doc:     "negative value",
			input:   "-1",
			expPort: 0,
			expErr:  `invalid port '-1': invalid syntax`,
		},
		// FIXME currently this is a valid port. I don't think it should be.
		// I'm leaving this test until we make a decision.
		// - erikh
		{
			doc:     "octal value",
			input:   "0123",
			expPort: 123,
		},
		{
			doc:     "max value",
			input:   "65535",
			expPort: 65535,
		},
		{
			doc:     "value out of range",
			input:   "65536",
			expPort: 0,
			expErr:  `invalid port '65536': value out of range`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			port, err := ParsePort(tc.input)
			if tc.expErr != "" {
				if err == nil || err.Error() != tc.expErr {
					t.Errorf("expected error '%s', got '%v'", tc.expErr, err.Error())
				}
			} else {
				if err != nil {
					t.Error(err)
				}
			}
			if port != tc.expPort {
				t.Errorf("expected port %d, got %d", tc.expPort, port)
			}

		})
	}
}

// TestParsePortRangeToInt tests behavior that's specific to [ParsePortRangeToInt],
// which is a shallow wrapper around [ParsePortRange], except for returning int's,
// and accepting empty values. Other cases are covered by [TestParsePortRange].
func TestParsePortRangeToInt(t *testing.T) {
	_, _, err := ParsePortRangeToInt("")
	if err != nil {
		t.Error(err)
	}
	begin, end, err := ParsePortRangeToInt("8000-9000")
	if err != nil {
		t.Error(err)
	}
	if expBegin := 8000; begin != 8000 {
		t.Errorf("expected begin %d, got %d", expBegin, begin)
	}
	if expEnd := 9000; end != expEnd {
		t.Errorf("expected end %d, got %d", expEnd, end)
	}
}

func TestPort(t *testing.T) {
	p, err := NewPort("tcp", "1234")
	if err != nil {
		t.Fatalf("tcp, 1234 had a parsing issue: %v", err)
	}

	if string(p) != "1234/tcp" {
		t.Fatal("tcp, 1234 did not result in the string 1234/tcp")
	}

	if p.Proto() != "tcp" {
		t.Fatal("protocol was not tcp")
	}

	if p.Port() != "1234" {
		t.Fatal("port string value was not 1234")
	}

	if p.Int() != 1234 {
		t.Fatal("port int value was not 1234")
	}

	_, err = NewPort("tcp", "asd1234")
	if err == nil {
		t.Fatal("tcp, asd1234 was supposed to fail")
	}

	_, err = NewPort("tcp", "1234-1230")
	if err == nil {
		t.Fatal("tcp, 1234-1230 was supposed to fail")
	}

	p, err = NewPort("tcp", "1234-1242")
	if err != nil {
		t.Fatalf("tcp, 1234-1242 had a parsing issue: %v", err)
	}

	if string(p) != "1234-1242/tcp" {
		t.Fatal("tcp, 1234-1242 did not result in the string 1234-1242/tcp")
	}
}

func TestSplitProtoPort(t *testing.T) {
	tests := []struct {
		doc      string
		input    string
		expPort  string
		expProto string
	}{
		{
			doc: "empty value",
		},
		{
			doc:      "zero value",
			input:    "0",
			expPort:  "0",
			expProto: "tcp",
		},
		{
			doc:      "empty port",
			input:    "/udp",
			expPort:  "",
			expProto: "",
		},
		{
			doc:      "single port",
			input:    "1234",
			expPort:  "1234",
			expProto: "tcp",
		},
		{
			doc:      "single port with empty protocol",
			input:    "1234/",
			expPort:  "1234",
			expProto: "tcp",
		},
		{
			doc:      "single port with protocol",
			input:    "1234/udp",
			expPort:  "1234",
			expProto: "udp",
		},
		{
			doc:      "port range",
			input:    "80-8080",
			expPort:  "80-8080",
			expProto: "tcp",
		},
		{
			doc:      "port range with empty protocol",
			input:    "80-8080/",
			expPort:  "80-8080",
			expProto: "tcp",
		},
		{
			doc:      "port range with protocol",
			input:    "80-8080/udp",
			expPort:  "80-8080",
			expProto: "udp",
		},

		// SplitProtoPort currently does not validate or normalize, so these are expected returns
		{
			doc:      "negative value",
			input:    "-1",
			expPort:  "-1",
			expProto: "tcp",
		},
		{
			doc:      "uppercase protocol",
			input:    "1234/UDP",
			expPort:  "1234",
			expProto: "UDP",
		},
		{
			doc:      "any value",
			input:    "any port value",
			expPort:  "any port value",
			expProto: "tcp",
		},
		{
			doc:      "any value with protocol",
			input:    "any port value/any proto value",
			expPort:  "any port value",
			expProto: "any proto value",
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			proto, port := SplitProtoPort(tc.input)
			if proto != tc.expProto {
				t.Errorf("expected proto %s, got %s", tc.expProto, proto)
			}
			if port != tc.expPort {
				t.Errorf("expected port %s, got %s", tc.expPort, port)
			}
		})
	}
}

func TestParsePortSpecFull(t *testing.T) {
	portMappings, err := ParsePortSpec("0.0.0.0:1234-1235:3333-3334/tcp")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	expected := []PortMapping{
		{
			Port: "3333/tcp",
			Binding: PortBinding{
				HostIP:   "0.0.0.0",
				HostPort: "1234",
			},
		},
		{
			Port: "3334/tcp",
			Binding: PortBinding{
				HostIP:   "0.0.0.0",
				HostPort: "1235",
			},
		},
	}

	if !reflect.DeepEqual(expected, portMappings) {
		t.Fatalf("wrong port mappings: got=%v, want=%v", portMappings, expected)
	}
}

func TestPartPortSpecIPV6(t *testing.T) {
	type test struct {
		name     string
		spec     string
		expected []PortMapping
	}
	cases := []test{
		{
			name: "square angled IPV6 without host port",
			spec: "[2001:4860:0:2001::68]::333",
			expected: []PortMapping{
				{
					Port: "333/tcp",
					Binding: PortBinding{
						HostIP:   "2001:4860:0:2001::68",
						HostPort: "",
					},
				},
			},
		},
		{
			name: "square angled IPV6 with host port",
			spec: "[::1]:80:80",
			expected: []PortMapping{
				{
					Port: "80/tcp",
					Binding: PortBinding{
						HostIP:   "::1",
						HostPort: "80",
					},
				},
			},
		},
		{
			name: "IPV6 without host port",
			spec: "2001:4860:0:2001::68::333",
			expected: []PortMapping{
				{
					Port: "333/tcp",
					Binding: PortBinding{
						HostIP:   "2001:4860:0:2001::68",
						HostPort: "",
					},
				},
			},
		},
		{
			name: "IPV6 with host port",
			spec: "::1:80:80",
			expected: []PortMapping{
				{
					Port: "80/tcp",
					Binding: PortBinding{
						HostIP:   "::1",
						HostPort: "80",
					},
				},
			},
		},
		{
			name: ":: IPV6, without host port",
			spec: "::::80",
			expected: []PortMapping{
				{
					Port: "80/tcp",
					Binding: PortBinding{
						HostIP:   "::",
						HostPort: "",
					},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			portMappings, err := ParsePortSpec(c.spec)
			if err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
			if !reflect.DeepEqual(c.expected, portMappings) {
				t.Fatalf("wrong port mappings: got=%v, want=%v", portMappings, c.expected)
			}
		})
	}
}

func TestParsePortSpecs(t *testing.T) {
	var (
		portMap    map[Port]struct{}
		bindingMap map[Port][]PortBinding
		err        error
	)

	portMap, bindingMap, err = ParsePortSpecs([]string{"1234/tcp", "2345/udp", "3456/sctp"})
	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portMap["1234/tcp"]; !ok {
		t.Fatal("1234/tcp was not parsed properly")
	}

	if _, ok := portMap["2345/udp"]; !ok {
		t.Fatal("2345/udp was not parsed properly")
	}

	if _, ok := portMap["3456/sctp"]; !ok {
		t.Fatal("3456/sctp was not parsed properly")
	}

	for portSpec, bindings := range bindingMap {
		if len(bindings) != 1 {
			t.Fatalf("%s should have exactly one binding", portSpec)
		}

		if bindings[0].HostIP != "" {
			t.Fatalf("HostIP should not be set for %s", portSpec)
		}

		if bindings[0].HostPort != "" {
			t.Fatalf("HostPort should not be set for %s", portSpec)
		}
	}

	portMap, bindingMap, err = ParsePortSpecs([]string{"1234:1234/tcp", "2345:2345/udp", "3456:3456/sctp"})
	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portMap["1234/tcp"]; !ok {
		t.Fatal("1234/tcp was not parsed properly")
	}

	if _, ok := portMap["2345/udp"]; !ok {
		t.Fatal("2345/udp was not parsed properly")
	}

	if _, ok := portMap["3456/sctp"]; !ok {
		t.Fatal("3456/sctp was not parsed properly")
	}

	for portSpec, bindings := range bindingMap {
		_, port := SplitProtoPort(string(portSpec))

		if len(bindings) != 1 {
			t.Fatalf("%s should have exactly one binding", portSpec)
		}

		if bindings[0].HostIP != "" {
			t.Fatalf("HostIP should not be set for %s", portSpec)
		}

		if bindings[0].HostPort != port {
			t.Fatalf("HostPort should be %s for %s", port, portSpec)
		}
	}

	portMap, bindingMap, err = ParsePortSpecs([]string{"0.0.0.0:1234:1234/tcp", "0.0.0.0:2345:2345/udp", "0.0.0.0:3456:3456/sctp"})
	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portMap["1234/tcp"]; !ok {
		t.Fatal("1234/tcp was not parsed properly")
	}

	if _, ok := portMap["2345/udp"]; !ok {
		t.Fatal("2345/udp was not parsed properly")
	}

	if _, ok := portMap["3456/sctp"]; !ok {
		t.Fatal("3456/sctp was not parsed properly")
	}

	for portSpec, bindings := range bindingMap {
		_, port := SplitProtoPort(string(portSpec))

		if len(bindings) != 1 {
			t.Fatalf("%s should have exactly one binding", portSpec)
		}

		if bindings[0].HostIP != "0.0.0.0" {
			t.Fatalf("HostIP is not 0.0.0.0 for %s", portSpec)
		}

		if bindings[0].HostPort != port {
			t.Fatalf("HostPort should be %s for %s", port, portSpec)
		}
	}

	_, _, err = ParsePortSpecs([]string{"localhost:1234:1234/tcp"})

	if err == nil {
		t.Fatal("Received no error while trying to parse a hostname instead of ip")
	}
}

func TestParsePortSpecsWithRange(t *testing.T) {
	var (
		portMap    map[Port]struct{}
		bindingMap map[Port][]PortBinding
		err        error
	)

	portMap, bindingMap, err = ParsePortSpecs([]string{"1234-1236/tcp", "2345-2347/udp", "3456-3458/sctp"})
	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portMap["1235/tcp"]; !ok {
		t.Fatal("1234/tcp was not parsed properly")
	}

	if _, ok := portMap["2346/udp"]; !ok {
		t.Fatal("2345/udp was not parsed properly")
	}

	if _, ok := portMap["3456/sctp"]; !ok {
		t.Fatal("3456/sctp was not parsed properly")
	}

	for portSpec, bindings := range bindingMap {
		if len(bindings) != 1 {
			t.Fatalf("%s should have exactly one binding", portSpec)
		}

		if bindings[0].HostIP != "" {
			t.Fatalf("HostIP should not be set for %s", portSpec)
		}

		if bindings[0].HostPort != "" {
			t.Fatalf("HostPort should not be set for %s", portSpec)
		}
	}

	portMap, bindingMap, err = ParsePortSpecs([]string{"1234-1236:1234-1236/tcp", "2345-2347:2345-2347/udp", "3456-3458:3456-3458/sctp"})
	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portMap["1235/tcp"]; !ok {
		t.Fatal("1234/tcp was not parsed properly")
	}

	if _, ok := portMap["2346/udp"]; !ok {
		t.Fatal("2345/udp was not parsed properly")
	}

	if _, ok := portMap["3456/sctp"]; !ok {
		t.Fatal("3456/sctp was not parsed properly")
	}

	for portSpec, bindings := range bindingMap {
		_, port := SplitProtoPort(string(portSpec))
		if len(bindings) != 1 {
			t.Fatalf("%s should have exactly one binding", portSpec)
		}

		if bindings[0].HostIP != "" {
			t.Fatalf("HostIP should not be set for %s", portSpec)
		}

		if bindings[0].HostPort != port {
			t.Fatalf("HostPort should be %s for %s", port, portSpec)
		}
	}

	portMap, bindingMap, err = ParsePortSpecs([]string{"0.0.0.0:1234-1236:1234-1236/tcp", "0.0.0.0:2345-2347:2345-2347/udp", "0.0.0.0:3456-3458:3456-3458/sctp"})
	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portMap["1235/tcp"]; !ok {
		t.Fatal("1234/tcp was not parsed properly")
	}

	if _, ok := portMap["2346/udp"]; !ok {
		t.Fatal("2345/udp was not parsed properly")
	}

	if _, ok := portMap["3456/sctp"]; !ok {
		t.Fatal("3456/sctp was not parsed properly")
	}

	for portSpec, bindings := range bindingMap {
		_, port := SplitProtoPort(string(portSpec))
		if len(bindings) != 1 || bindings[0].HostIP != "0.0.0.0" || bindings[0].HostPort != port {
			t.Fatalf("Expect single binding to port %s but found %s", port, bindings)
		}
	}

	_, _, err = ParsePortSpecs([]string{"localhost:1234-1236:1234-1236/tcp"})

	if err == nil {
		t.Fatal("Received no error while trying to parse a hostname instead of ip")
	}
}

func TestParseNetworkOptsPrivateOnly(t *testing.T) {
	ports, bindings, err := ParsePortSpecs([]string{"192.168.1.100::80"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 {
		t.Logf("Expected 1 got %d", len(ports))
		t.FailNow()
	}
	if len(bindings) != 1 {
		t.Logf("Expected 1 got %d", len(bindings))
		t.FailNow()
	}
	for k := range ports {
		if k.Proto() != "tcp" {
			t.Logf("Expected tcp got %s", k.Proto())
			t.Fail()
		}
		if k.Port() != "80" {
			t.Logf("Expected 80 got %s", k.Port())
			t.Fail()
		}
		b, exists := bindings[k]
		if !exists {
			t.Log("Binding does not exist")
			t.FailNow()
		}
		if len(b) != 1 {
			t.Logf("Expected 1 got %d", len(b))
			t.FailNow()
		}
		s := b[0]
		if s.HostPort != "" {
			t.Logf("Expected \"\" got %s", s.HostPort)
			t.Fail()
		}
		if s.HostIP != "192.168.1.100" {
			t.Fail()
		}
	}
}

func TestParseNetworkOptsPublic(t *testing.T) {
	ports, bindings, err := ParsePortSpecs([]string{"192.168.1.100:8080:80"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 {
		t.Logf("Expected 1 got %d", len(ports))
		t.FailNow()
	}
	if len(bindings) != 1 {
		t.Logf("Expected 1 got %d", len(bindings))
		t.FailNow()
	}
	for k := range ports {
		if k.Proto() != "tcp" {
			t.Logf("Expected tcp got %s", k.Proto())
			t.Fail()
		}
		if k.Port() != "80" {
			t.Logf("Expected 80 got %s", k.Port())
			t.Fail()
		}
		b, exists := bindings[k]
		if !exists {
			t.Log("Binding does not exist")
			t.FailNow()
		}
		if len(b) != 1 {
			t.Logf("Expected 1 got %d", len(b))
			t.FailNow()
		}
		s := b[0]
		if s.HostPort != "8080" {
			t.Logf("Expected 8080 got %s", s.HostPort)
			t.Fail()
		}
		if s.HostIP != "192.168.1.100" {
			t.Fail()
		}
	}
}

func TestParseNetworkOptsPublicNoPort(t *testing.T) {
	ports, bindings, err := ParsePortSpecs([]string{"192.168.1.100"})

	if err == nil {
		t.Logf("Expected error Invalid containerPort")
		t.Fail()
	}
	if ports != nil {
		t.Logf("Expected nil got %s", ports)
		t.Fail()
	}
	if bindings != nil {
		t.Logf("Expected nil got %s", bindings)
		t.Fail()
	}
}

func TestParseNetworkOptsNegativePorts(t *testing.T) {
	ports, bindings, err := ParsePortSpecs([]string{"192.168.1.100:-1:-1"})

	if err == nil {
		t.Fail()
	}
	if len(ports) != 0 {
		t.Logf("Expected nil got %d", len(ports))
		t.Fail()
	}
	if len(bindings) != 0 {
		t.Logf("Expected 0 got %d", len(bindings))
		t.Fail()
	}
}

func TestParseNetworkOptsUdp(t *testing.T) {
	ports, bindings, err := ParsePortSpecs([]string{"192.168.1.100::6000/udp"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 {
		t.Logf("Expected 1 got %d", len(ports))
		t.FailNow()
	}
	if len(bindings) != 1 {
		t.Logf("Expected 1 got %d", len(bindings))
		t.FailNow()
	}
	for k := range ports {
		if k.Proto() != "udp" {
			t.Logf("Expected udp got %s", k.Proto())
			t.Fail()
		}
		if k.Port() != "6000" {
			t.Logf("Expected 6000 got %s", k.Port())
			t.Fail()
		}
		b, exists := bindings[k]
		if !exists {
			t.Log("Binding does not exist")
			t.FailNow()
		}
		if len(b) != 1 {
			t.Logf("Expected 1 got %d", len(b))
			t.FailNow()
		}
		s := b[0]
		if s.HostPort != "" {
			t.Logf("Expected \"\" got %s", s.HostPort)
			t.Fail()
		}
		if s.HostIP != "192.168.1.100" {
			t.Fail()
		}
	}
}

func TestParseNetworkOptsSctp(t *testing.T) {
	ports, bindings, err := ParsePortSpecs([]string{"192.168.1.100::6000/sctp"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 1 {
		t.Logf("Expected 1 got %d", len(ports))
		t.FailNow()
	}
	if len(bindings) != 1 {
		t.Logf("Expected 1 got %d", len(bindings))
		t.FailNow()
	}
	for k := range ports {
		if k.Proto() != "sctp" {
			t.Logf("Expected sctp got %s", k.Proto())
			t.Fail()
		}
		if k.Port() != "6000" {
			t.Logf("Expected 6000 got %s", k.Port())
			t.Fail()
		}
		b, exists := bindings[k]
		if !exists {
			t.Log("Binding does not exist")
			t.FailNow()
		}
		if len(b) != 1 {
			t.Logf("Expected 1 got %d", len(b))
			t.FailNow()
		}
		s := b[0]
		if s.HostPort != "" {
			t.Logf("Expected \"\" got %s", s.HostPort)
			t.Fail()
		}
		if s.HostIP != "192.168.1.100" {
			t.Fail()
		}
	}
}

func TestStringer(t *testing.T) {
	tests := []struct {
		doc      string
		in       string
		expected string
	}{
		{
			doc:      "no host mapping",
			in:       ":8080:6000/tcp",
			expected: ":8080:6000/tcp",
		},
		{
			doc:      "no proto",
			in:       "192.168.1.100:8080:6000",
			expected: "192.168.1.100:8080:6000/tcp",
		},
		{
			doc:      "no host port",
			in:       "192.168.1.100::6000/udp",
			expected: "192.168.1.100::6000/udp",
		},
		{
			doc:      "no mapping, port, or proto",
			in:       "::6000",
			expected: "::6000/tcp",
		},
		{
			doc:      "ipv4 mapping",
			in:       "192.168.1.100:8080:6000/udp",
			expected: "192.168.1.100:8080:6000/udp",
		},
		{
			doc:      "ipv4 mapping without host port",
			in:       "192.168.1.100::6000/udp",
			expected: "192.168.1.100::6000/udp",
		},
		{
			doc:      "ipv6 mapping",
			in:       "[::1]:8080:6000/udp",
			expected: "[::1]:8080:6000/udp",
		},
		{
			doc:      "ipv6 mapping without host port",
			in:       "[::1]::6000/udp",
			expected: "[::1]::6000/udp",
		},
		{
			doc:      "ipv6 legacy mapping",
			in:       "::1:8080:6000/udp",
			expected: "[::1]:8080:6000/udp",
		},
		{
			doc:      "ipv6 legacy mapping without host port",
			in:       "::::6000/udp",
			expected: "[::]::6000/udp",
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			mappings, err := ParsePortSpec(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if len(mappings) != 1 {
				// All tests produce a single mapping
				t.Fatalf("Expected 1 got %d", len(mappings))
			}
			if actual := mappings[0].String(); actual != tc.expected {
				t.Errorf("Expected %s got %s", tc.expected, actual)
			}
		})
	}
}
