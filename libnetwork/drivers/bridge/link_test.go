//go:build linux

package bridge

import (
	"net"
	"testing"

	"github.com/docker/docker/libnetwork/types"
)

func getPorts() []types.TransportPort {
	return []types.TransportPort{
		{Proto: types.TCP, Port: uint16(5000)},
		{Proto: types.UDP, Port: uint16(400)},
		{Proto: types.TCP, Port: uint16(600)},
	}
}

func TestLinkNew(t *testing.T) {
	ports := getPorts()

	const (
		pIP        = "172.0.17.3"
		cIP        = "172.0.17.2"
		bridgeName = "docker0"
	)

	parentIP := net.ParseIP(pIP)
	childIP := net.ParseIP(cIP)

	l, err := newLink(parentIP, childIP, ports, bridgeName)
	if err != nil {
		t.Errorf("unexpected error from newlink(): %v", err)
	}
	if l == nil {
		t.FailNow()
	}
	if l.parentIP.String() != pIP {
		t.Fail()
	}
	if l.childIP.String() != cIP {
		t.Fail()
	}
	for i, p := range l.ports {
		if p != ports[i] {
			t.Fail()
		}
	}
	if l.bridge != bridgeName {
		t.Fail()
	}
}
