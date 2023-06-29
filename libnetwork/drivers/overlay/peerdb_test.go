//go:build linux

package overlay

import (
	"net"
	"testing"
)

func TestPeerMarshal(t *testing.T) {
	_, ipNet, _ := net.ParseCIDR("192.168.0.1/24")
	p := &peerEntry{
		eid:        "eid",
		isLocal:    true,
		peerIPMask: ipNet.Mask,
		vtep:       ipNet.IP,
	}
	entryDB := p.MarshalDB()
	x := entryDB.UnMarshalDB()
	if x.eid != p.eid {
		t.Fatalf("Incorrect Unmarshalling for eid: %v != %v", x.eid, p.eid)
	}
	if x.isLocal != p.isLocal {
		t.Fatalf("Incorrect Unmarshalling for isLocal: %v != %v", x.isLocal, p.isLocal)
	}
	if x.peerIPMask.String() != p.peerIPMask.String() {
		t.Fatalf("Incorrect Unmarshalling for eid: %v != %v", x.peerIPMask, p.peerIPMask)
	}
	if x.vtep.String() != p.vtep.String() {
		t.Fatalf("Incorrect Unmarshalling for eid: %v != %v", x.vtep, p.vtep)
	}
}
