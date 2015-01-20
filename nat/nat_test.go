package nat

import (
	"testing"
)

func TestParsePort(t *testing.T) {
	var (
		p   int
		err error
	)

	p, err = ParsePort("1234")

	if err != nil || p != 1234 {
		t.Fatal("Parsing '1234' did not succeed")
	}

	// FIXME currently this is a valid port. I don't think it should be.
	// I'm leaving this test commented out until we make a decision.
	// - erikh

	/*
		p, err = ParsePort("0123")

		if err != nil {
		    t.Fatal("Successfully parsed port '0123' to '123'")
		}
	*/

	p, err = ParsePort("asdf")

	if err == nil || p != 0 {
		t.Fatal("Parsing port 'asdf' succeeded")
	}

	p, err = ParsePort("1asdf")

	if err == nil || p != 0 {
		t.Fatal("Parsing port '1asdf' succeeded")
	}
}

func TestPort(t *testing.T) {
	p := NewPort("tcp", "1234")

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
}

func TestSplitProtoPort(t *testing.T) {
	var (
		proto string
		port  string
	)

	proto, port = SplitProtoPort("1234/tcp")

	if proto != "tcp" || port != "1234" {
		t.Fatal("Could not split 1234/tcp properly")
	}

	proto, port = SplitProtoPort("")

	if proto != "" || port != "" {
		t.Fatal("parsing an empty string yielded surprising results", proto, port)
	}

	proto, port = SplitProtoPort("1234")

	if proto != "tcp" || port != "1234" {
		t.Fatal("tcp is not the default protocol for portspec '1234'", proto, port)
	}

	proto, port = SplitProtoPort("1234/")

	if proto != "tcp" || port != "1234" {
		t.Fatal("parsing '1234/' yielded:" + port + "/" + proto)
	}

	proto, port = SplitProtoPort("/tcp")

	if proto != "" || port != "" {
		t.Fatal("parsing '/tcp' yielded:" + port + "/" + proto)
	}
}

func TestParsePortSpecs(t *testing.T) {
	var (
		portMap    map[Port]struct{}
		bindingMap map[Port][]PortBinding
		err        error
	)

	portMap, bindingMap, err = ParsePortSpecs([]string{"1234/tcp", "2345/udp"})

	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portMap[Port("1234/tcp")]; !ok {
		t.Fatal("1234/tcp was not parsed properly")
	}

	if _, ok := portMap[Port("2345/udp")]; !ok {
		t.Fatal("2345/udp was not parsed properly")
	}

	for portspec, bindings := range bindingMap {
		if len(bindings) != 1 {
			t.Fatalf("%s should have exactly one binding", portspec)
		}

		if bindings[0].HostIp != "" {
			t.Fatalf("HostIp should not be set for %s", portspec)
		}

		if bindings[0].HostPort != "" {
			t.Fatalf("HostPort should not be set for %s", portspec)
		}
	}

	portMap, bindingMap, err = ParsePortSpecs([]string{"1234:1234/tcp", "2345:2345/udp"})

	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portMap[Port("1234/tcp")]; !ok {
		t.Fatal("1234/tcp was not parsed properly")
	}

	if _, ok := portMap[Port("2345/udp")]; !ok {
		t.Fatal("2345/udp was not parsed properly")
	}

	for portspec, bindings := range bindingMap {
		_, port := SplitProtoPort(string(portspec))

		if len(bindings) != 1 {
			t.Fatalf("%s should have exactly one binding", portspec)
		}

		if bindings[0].HostIp != "" {
			t.Fatalf("HostIp should not be set for %s", portspec)
		}

		if bindings[0].HostPort != port {
			t.Fatalf("HostPort should be %s for %s", port, portspec)
		}
	}

	portMap, bindingMap, err = ParsePortSpecs([]string{"0.0.0.0:1234:1234/tcp", "0.0.0.0:2345:2345/udp"})

	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portMap[Port("1234/tcp")]; !ok {
		t.Fatal("1234/tcp was not parsed properly")
	}

	if _, ok := portMap[Port("2345/udp")]; !ok {
		t.Fatal("2345/udp was not parsed properly")
	}

	for portspec, bindings := range bindingMap {
		_, port := SplitProtoPort(string(portspec))

		if len(bindings) != 1 {
			t.Fatalf("%s should have exactly one binding", portspec)
		}

		if bindings[0].HostIp != "0.0.0.0" {
			t.Fatalf("HostIp is not 0.0.0.0 for %s", portspec)
		}

		if bindings[0].HostPort != port {
			t.Fatalf("HostPort should be %s for %s", port, portspec)
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

	portMap, bindingMap, err = ParsePortSpecs([]string{"1234-1236/tcp", "2345-2347/udp"})

	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portMap[Port("1235/tcp")]; !ok {
		t.Fatal("1234/tcp was not parsed properly")
	}

	if _, ok := portMap[Port("2346/udp")]; !ok {
		t.Fatal("2345/udp was not parsed properly")
	}

	for portspec, bindings := range bindingMap {
		if len(bindings) != 1 {
			t.Fatalf("%s should have exactly one binding", portspec)
		}

		if bindings[0].HostIp != "" {
			t.Fatalf("HostIp should not be set for %s", portspec)
		}

		if bindings[0].HostPort != "" {
			t.Fatalf("HostPort should not be set for %s", portspec)
		}
	}

	portMap, bindingMap, err = ParsePortSpecs([]string{"1234-1236:1234-1236/tcp", "2345-2347:2345-2347/udp"})

	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portMap[Port("1235/tcp")]; !ok {
		t.Fatal("1234/tcp was not parsed properly")
	}

	if _, ok := portMap[Port("2346/udp")]; !ok {
		t.Fatal("2345/udp was not parsed properly")
	}

	for portspec, bindings := range bindingMap {
		_, port := SplitProtoPort(string(portspec))
		if len(bindings) != 1 {
			t.Fatalf("%s should have exactly one binding", portspec)
		}

		if bindings[0].HostIp != "" {
			t.Fatalf("HostIp should not be set for %s", portspec)
		}

		if bindings[0].HostPort != port {
			t.Fatalf("HostPort should be %s for %s", port, portspec)
		}
	}

	portMap, bindingMap, err = ParsePortSpecs([]string{"0.0.0.0:1234-1236:1234-1236/tcp", "0.0.0.0:2345-2347:2345-2347/udp"})

	if err != nil {
		t.Fatalf("Error while processing ParsePortSpecs: %s", err)
	}

	if _, ok := portMap[Port("1235/tcp")]; !ok {
		t.Fatal("1234/tcp was not parsed properly")
	}

	if _, ok := portMap[Port("2346/udp")]; !ok {
		t.Fatal("2345/udp was not parsed properly")
	}

	for portspec, bindings := range bindingMap {
		_, port := SplitProtoPort(string(portspec))
		if len(bindings) != 1 || bindings[0].HostIp != "0.0.0.0" || bindings[0].HostPort != port {
			t.Fatalf("Expect single binding to port %s but found %s", port, bindings)
		}
	}

	_, _, err = ParsePortSpecs([]string{"localhost:1234-1236:1234-1236/tcp"})

	if err == nil {
		t.Fatal("Received no error while trying to parse a hostname instead of ip")
	}
}
