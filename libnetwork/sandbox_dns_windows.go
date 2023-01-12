//go:build windows
// +build windows

package libnetwork

import (
	"github.com/docker/docker/libnetwork/etchosts"
)

// Stub implementations for DNS related functions

func (sb *Sandbox) startResolver(bool) {}

func (sb *Sandbox) setupResolutionFiles() error {
	return nil
}

func (sb *Sandbox) restorePath() {}

func (sb *Sandbox) updateHostsFile(ifaceIP []string) error {
	return nil
}

func (sb *Sandbox) addHostsEntries(recs []etchosts.Record) {}

func (sb *Sandbox) deleteHostsEntries(recs []etchosts.Record) {}

func (sb *Sandbox) updateDNS(ipv6Enabled bool) error {
	return nil
}

func (sb *Sandbox) setupDNS() error {
	return nil
}

func (sb *Sandbox) rebuildDNS() error {
	return nil
}
