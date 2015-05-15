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

const defaultHeartbeat = 10

// JoinCallback provides a callback event for new node joining the cluster
type JoinCallback func(entries []net.IP)

// LeaveCallback provides a callback event for node leaving the cluster
type LeaveCallback func(entries []net.IP)

// HostDiscovery primary interface
type HostDiscovery interface {
	// StartDiscovery initiates the discovery process and provides appropriate callbacks
	StartDiscovery(*config.ClusterCfg, JoinCallback, LeaveCallback) error
	// StopDiscovery stops the discovery perocess
	StopDiscovery() error
	// Fetch returns a list of host IPs that are currently discovered
	Fetch() ([]net.IP, error)
}

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

	hb := cfg.Heartbeat
	if hb == 0 {
		hb = defaultHeartbeat
	}
	d, err := discovery.New(cfg.Discovery, hb)
	if err != nil {
		return err
	}

	if ip := net.ParseIP(cfg.Address); ip == nil {
		return errors.New("Address config should be either ipv4 or ipv6 address")
	}

	if err := d.Register(cfg.Address + ":0"); err != nil {
		return err
	}

	h.Lock()
	h.discovery = d
	h.Unlock()

	go d.Watch(func(entries []*discovery.Entry) {
		h.processCallback(entries, joinCallback, leaveCallback)
	})

	go sustainHeartbeat(d, hb, cfg, h.stopChan)
	return nil
}

func (h *hostDiscovery) StopDiscovery() error {
	h.Lock()
	stopChan := h.stopChan
	h.discovery = nil
	h.Unlock()

	close(stopChan)
	return nil
}

func sustainHeartbeat(d discovery.Discovery, hb uint64, config *config.ClusterCfg, stopChan chan struct{}) {
	for {
		select {
		case <-stopChan:
			return
		case <-time.After(time.Duration(hb) * time.Second):
			if err := d.Register(config.Address + ":0"); err != nil {
				log.Warn(err)
			}
		}
	}
}

func (h *hostDiscovery) processCallback(entries []*discovery.Entry, joinCallback JoinCallback, leaveCallback LeaveCallback) {
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
	hd := h.discovery
	h.Unlock()
	if hd == nil {
		return nil, errors.New("No Active Discovery")
	}
	entries, err := hd.Fetch()
	if err != nil {
		return nil, err
	}
	ips := []net.IP{}
	for _, entry := range entries {
		ips = append(ips, net.ParseIP(entry.Host))
	}
	return ips, nil
}

func hosts(entries []*discovery.Entry) mapset.Set {
	hosts := mapset.NewSet()
	for _, entry := range entries {
		hosts.Add(entry.Host)
	}
	return hosts
}
