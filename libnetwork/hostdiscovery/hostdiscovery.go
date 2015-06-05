// +build libnetwork_discovery

package hostdiscovery

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	mapset "github.com/deckarep/golang-set"
	"github.com/docker/libnetwork/config"
	"github.com/docker/swarm/discovery"
	// Anonymous import will be removed after we upgrade to latest swarm
	_ "github.com/docker/swarm/discovery/file"
	// Anonymous import will be removed after we upgrade to latest swarm
	_ "github.com/docker/swarm/discovery/kv"
	// Anonymous import will be removed after we upgrade to latest swarm
	_ "github.com/docker/swarm/discovery/nodes"
	// Anonymous import will be removed after we upgrade to latest swarm
	_ "github.com/docker/swarm/discovery/token"
)

const defaultHeartbeat = time.Duration(10) * time.Second
const TTLFactor = 3

type hostDiscovery struct {
	discovery discovery.Discovery
	nodes     mapset.Set
	stopChan  chan struct{}
	sync.Mutex
}

// NewHostDiscovery function creates a host discovery object
func NewHostDiscovery() HostDiscovery {
	return &hostDiscovery{nodes: mapset.NewSet(), stopChan: make(chan struct{})}
}

func (h *hostDiscovery) StartDiscovery(cfg *config.ClusterCfg, joinCallback JoinCallback, leaveCallback LeaveCallback) error {
	if cfg == nil {
		return fmt.Errorf("discovery requires a valid configuration")
	}

	hb := time.Duration(cfg.Heartbeat) * time.Second
	if hb == 0 {
		hb = defaultHeartbeat
	}
	d, err := discovery.New(cfg.Discovery, hb, TTLFactor*hb)
	if err != nil {
		return err
	}

	if ip := net.ParseIP(cfg.Address); ip == nil {
		return errors.New("address config should be either ipv4 or ipv6 address")
	}

	if err := d.Register(cfg.Address + ":0"); err != nil {
		return err
	}

	h.Lock()
	h.discovery = d
	h.Unlock()

	discoveryCh, errCh := d.Watch(h.stopChan)
	go h.monitorDiscovery(discoveryCh, errCh, joinCallback, leaveCallback)
	go h.sustainHeartbeat(d, hb, cfg)
	return nil
}

func (h *hostDiscovery) monitorDiscovery(ch <-chan discovery.Entries, errCh <-chan error, joinCallback JoinCallback, leaveCallback LeaveCallback) {
	for {
		select {
		case entries := <-ch:
			h.processCallback(entries, joinCallback, leaveCallback)
		case err := <-errCh:
			log.Errorf("discovery error: %v", err)
		case <-h.stopChan:
			return
		}
	}
}

func (h *hostDiscovery) StopDiscovery() error {
	h.Lock()
	stopChan := h.stopChan
	h.discovery = nil
	h.Unlock()

	close(stopChan)
	return nil
}

func (h *hostDiscovery) sustainHeartbeat(d discovery.Discovery, hb time.Duration, config *config.ClusterCfg) {
	for {
		select {
		case <-h.stopChan:
			return
		case <-time.After(hb):
			if err := d.Register(config.Address + ":0"); err != nil {
				log.Warn(err)
			}
		}
	}
}

func (h *hostDiscovery) processCallback(entries discovery.Entries, joinCallback JoinCallback, leaveCallback LeaveCallback) {
	updated := hosts(entries)
	h.Lock()
	existing := h.nodes
	added, removed := diff(existing, updated)
	h.nodes = updated
	h.Unlock()

	if len(added) > 0 {
		joinCallback(added)
	}
	if len(removed) > 0 {
		leaveCallback(removed)
	}
}

func diff(existing mapset.Set, updated mapset.Set) (added []net.IP, removed []net.IP) {
	addSlice := updated.Difference(existing).ToSlice()
	removeSlice := existing.Difference(updated).ToSlice()
	for _, ip := range addSlice {
		added = append(added, net.ParseIP(ip.(string)))
	}
	for _, ip := range removeSlice {
		removed = append(removed, net.ParseIP(ip.(string)))
	}
	return
}

func (h *hostDiscovery) Fetch() ([]net.IP, error) {
	h.Lock()
	defer h.Unlock()
	ips := []net.IP{}
	for _, ipstr := range h.nodes.ToSlice() {
		ips = append(ips, net.ParseIP(ipstr.(string)))
	}
	return ips, nil
}

func hosts(entries discovery.Entries) mapset.Set {
	hosts := mapset.NewSet()
	for _, entry := range entries {
		hosts.Add(entry.Host)
	}
	return hosts
}
