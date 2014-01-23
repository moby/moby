package portallocator

import (
	"github.com/dotcloud/docker/pkg/collections"
	"testing"
)

func reset() {
	lock.Lock()
	defer lock.Unlock()

	allocatedPorts = portMappings{}
	availablePorts = portMappings{}

	allocatedPorts["udp"] = collections.NewOrderedIntSet()
	availablePorts["udp"] = collections.NewOrderedIntSet()
	allocatedPorts["tcp"] = collections.NewOrderedIntSet()
	availablePorts["tcp"] = collections.NewOrderedIntSet()
}

func TestRequestNewPort(t *testing.T) {
	defer reset()

	port, err := RequestPort("tcp", 0)
	if err != nil {
		t.Fatal(err)
	}

	if expected := BeginPortRange; port != expected {
		t.Fatalf("Expected port %d got %d", expected, port)
	}
}

func TestRequestSpecificPort(t *testing.T) {
	defer reset()

	port, err := RequestPort("tcp", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if port != 5000 {
		t.Fatalf("Expected port 5000 got %d", port)
	}
}

func TestReleasePort(t *testing.T) {
	defer reset()

	port, err := RequestPort("tcp", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if port != 5000 {
		t.Fatalf("Expected port 5000 got %d", port)
	}

	if err := ReleasePort("tcp", 5000); err != nil {
		t.Fatal(err)
	}
}

func TestReuseReleasedPort(t *testing.T) {
	defer reset()

	port, err := RequestPort("tcp", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if port != 5000 {
		t.Fatalf("Expected port 5000 got %d", port)
	}

	if err := ReleasePort("tcp", 5000); err != nil {
		t.Fatal(err)
	}

	port, err = RequestPort("tcp", 5000)
	if err != nil {
		t.Fatal(err)
	}
}

func TestReleaseUnreadledPort(t *testing.T) {
	defer reset()

	port, err := RequestPort("tcp", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if port != 5000 {
		t.Fatalf("Expected port 5000 got %d", port)
	}

	port, err = RequestPort("tcp", 5000)
	if err != ErrPortAlreadyAllocated {
		t.Fatalf("Expected error %s got %s", ErrPortAlreadyAllocated, err)
	}
}

func TestUnknowProtocol(t *testing.T) {
	defer reset()

	if _, err := RequestPort("tcpp", 0); err != ErrUnknownProtocol {
		t.Fatalf("Expected error %s got %s", ErrUnknownProtocol, err)
	}
}

func TestAllocateAllPorts(t *testing.T) {
	defer reset()

	for i := 0; i <= EndPortRange-BeginPortRange; i++ {
		port, err := RequestPort("tcp", 0)
		if err != nil {
			t.Fatal(err)
		}

		if expected := BeginPortRange + i; port != expected {
			t.Fatalf("Expected port %d got %d", expected, port)
		}
	}

	if _, err := RequestPort("tcp", 0); err != ErrPortExceedsRange {
		t.Fatalf("Expected error %s got %s", ErrPortExceedsRange, err)
	}

	_, err := RequestPort("udp", 0)
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkAllocatePorts(b *testing.B) {
	defer reset()

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		for i := 0; i <= EndPortRange-BeginPortRange; i++ {
			port, err := RequestPort("tcp", 0)
			if err != nil {
				b.Fatal(err)
			}

			if expected := BeginPortRange + i; port != expected {
				b.Fatalf("Expected port %d got %d", expected, port)
			}
		}
		reset()
	}
	b.StopTimer()
}
