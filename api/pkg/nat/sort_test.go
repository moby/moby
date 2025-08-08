package nat

import (
	"reflect"
	"testing"
)

func TestSortUniquePorts(t *testing.T) {
	ports := []Port{
		"6379/tcp",
		"22/tcp",
	}

	Sort(ports, func(ip, jp Port) bool {
		return ip.Int() < jp.Int() || (ip.Int() == jp.Int() && ip.Proto() == "tcp")
	})

	first := ports[0]
	if string(first) != "22/tcp" {
		t.Log(first)
		t.Fail()
	}
}

func TestSortSamePortWithDifferentProto(t *testing.T) {
	ports := []Port{
		"8888/tcp",
		"8888/udp",
		"6379/tcp",
		"6379/udp",
	}

	Sort(ports, func(ip, jp Port) bool {
		return ip.Int() < jp.Int() || (ip.Int() == jp.Int() && ip.Proto() == "tcp")
	})

	first := ports[0]
	if string(first) != "6379/tcp" {
		t.Fail()
	}
}

func TestSortPortMap(t *testing.T) {
	ports := []Port{
		"22/tcp",
		"22/udp",
		"8000/tcp",
		"8443/tcp",
		"6379/tcp",
		"9999/tcp",
	}

	portMap := PortMap{
		"22/tcp":   []PortBinding{{}},
		"8000/tcp": []PortBinding{{}},
		"8443/tcp": []PortBinding{},
		"6379/tcp": []PortBinding{{}, {HostIP: "0.0.0.0", HostPort: "32749"}},
		"9999/tcp": []PortBinding{{HostIP: "0.0.0.0", HostPort: "40000"}},
	}

	SortPortMap(ports, portMap)
	if !reflect.DeepEqual(ports, []Port{
		"9999/tcp",
		"6379/tcp",
		"8443/tcp",
		"8000/tcp",
		"22/tcp",
		"22/udp",
	}) {
		t.Errorf("failed to prioritize port with explicit mappings, got %v", ports)
	}
	if pm := portMap[Port("6379/tcp")]; !reflect.DeepEqual(pm, []PortBinding{
		{HostIP: "0.0.0.0", HostPort: "32749"},
		{},
	}) {
		t.Errorf("failed to prioritize bindings with explicit mappings, got %v", pm)
	}
}
