package docker

import (
	"fmt"
	"testing"
)

func TestSortUniquePorts(t *testing.T) {
	ports := []Port{
		Port("6379/tcp"),
		Port("22/tcp"),
	}

	sortPorts(ports, func(ip, jp Port) bool {
		return ip.Int() < jp.Int() || (ip.Int() == jp.Int() && ip.Proto() == "tcp")
	})

	first := ports[0]
	if fmt.Sprint(first) != "22/tcp" {
		t.Log(fmt.Sprint(first))
		t.Fail()
	}
}

func TestSortSamePortWithDifferentProto(t *testing.T) {
	ports := []Port{
		Port("8888/tcp"),
		Port("8888/udp"),
		Port("6379/tcp"),
		Port("6379/udp"),
	}

	sortPorts(ports, func(ip, jp Port) bool {
		return ip.Int() < jp.Int() || (ip.Int() == jp.Int() && ip.Proto() == "tcp")
	})

	first := ports[0]
	if fmt.Sprint(first) != "6379/tcp" {
		t.Fail()
	}
}
