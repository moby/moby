// +build libnetwork_discovery

package hostdiscovery

import (
	"net"
	"testing"
	"time"

	mapset "github.com/deckarep/golang-set"
	_ "github.com/docker/libnetwork/netutils"

	"github.com/docker/libnetwork/config"
	"github.com/docker/swarm/discovery"
)

func TestDiscovery(t *testing.T) {
	_, err := net.Dial("tcp", "discovery-stage.hub.docker.com:80")
	if err != nil {
		t.Skip("Skipping Discovery test which need connectivity to discovery-stage.hub.docker.com")
	}

	hd := NewHostDiscovery()
	config, err := config.ParseConfig("libnetwork.toml")
	if err != nil {
		t.Fatal(err)
	}

	err = hd.StartDiscovery(&config.Cluster, func(hosts []net.IP) {}, func(hosts []net.IP) {})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Duration(config.Cluster.Heartbeat*2) * time.Second)
	hosts, err := hd.Fetch()
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, ip := range hosts {
		if ip.Equal(net.ParseIP(config.Cluster.Address)) {
			found = true
		}
	}
	if !found {
		t.Fatalf("Expecting hosts. But none discovered ")
	}
	err = hd.StopDiscovery()
	if err != nil {
		t.Fatal(err)
	}
}

func TestBadDiscovery(t *testing.T) {
	_, err := net.Dial("tcp", "discovery-stage.hub.docker.com:80")
	if err != nil {
		t.Skip("Skipping Discovery test which need connectivity to discovery-stage.hub.docker.com")
	}

	hd := NewHostDiscovery()
	cfg := &config.Config{}
	cfg.Cluster.Discovery = ""
	err = hd.StartDiscovery(&cfg.Cluster, func(hosts []net.IP) {}, func(hosts []net.IP) {})
	if err == nil {
		t.Fatal("Invalid discovery configuration must fail")
	}
	cfg, err = config.ParseConfig("libnetwork.toml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Cluster.Address = "invalid"
	err = hd.StartDiscovery(&cfg.Cluster, func(hosts []net.IP) {}, func(hosts []net.IP) {})
	if err == nil {
		t.Fatal("Invalid discovery address configuration must fail")
	}
}

func TestDiff(t *testing.T) {
	existing := mapset.NewSetFromSlice([]interface{}{"1.1.1.1", "2.2.2.2"})
	addedIP := "3.3.3.3"
	updated := existing.Clone()
	updated.Add(addedIP)

	added, removed := diff(existing, updated)
	if len(added) != 1 {
		t.Fatalf("Diff failed for an Add update. Expecting 1 element, but got %d elements", len(added))
	}
	if added[0].String() != addedIP {
		t.Fatalf("Expecting : %v, Got : %v", addedIP, added[0])
	}
	if len(removed) > 0 {
		t.Fatalf("Diff failed for remove use-case. Expecting 0 element, but got %d elements", len(removed))
	}

	updated = mapset.NewSetFromSlice([]interface{}{addedIP})
	added, removed = diff(existing, updated)
	if len(removed) != 2 {
		t.Fatalf("Diff failed for an remove update. Expecting 2 element, but got %d elements", len(removed))
	}
	if len(added) != 1 {
		t.Fatalf("Diff failed for add use-case. Expecting 1 element, but got %d elements", len(added))
	}
}

func TestAddedCallback(t *testing.T) {
	hd := hostDiscovery{}
	hd.nodes = mapset.NewSetFromSlice([]interface{}{"1.1.1.1"})
	update := []*discovery.Entry{&discovery.Entry{Host: "1.1.1.1", Port: "0"}, &discovery.Entry{Host: "2.2.2.2", Port: "0"}}

	added := false
	removed := false
	hd.processCallback(update, func(hosts []net.IP) { added = true }, func(hosts []net.IP) { removed = true })
	if !added {
		t.Fatalf("Expecting a Added callback notification. But none received")
	}
}

func TestRemovedCallback(t *testing.T) {
	hd := hostDiscovery{}
	hd.nodes = mapset.NewSetFromSlice([]interface{}{"1.1.1.1", "2.2.2.2"})
	update := []*discovery.Entry{&discovery.Entry{Host: "1.1.1.1", Port: "0"}}

	added := false
	removed := false
	hd.processCallback(update, func(hosts []net.IP) { added = true }, func(hosts []net.IP) { removed = true })
	if !removed {
		t.Fatalf("Expecting a Removed callback notification. But none received")
	}
}

func TestNoCallback(t *testing.T) {
	hd := hostDiscovery{}
	hd.nodes = mapset.NewSetFromSlice([]interface{}{"1.1.1.1", "2.2.2.2"})
	update := []*discovery.Entry{&discovery.Entry{Host: "1.1.1.1", Port: "0"}, &discovery.Entry{Host: "2.2.2.2", Port: "0"}}

	added := false
	removed := false
	hd.processCallback(update, func(hosts []net.IP) { added = true }, func(hosts []net.IP) { removed = true })
	if added || removed {
		t.Fatalf("Not expecting any callback notification. But received a callback")
	}
}
