//go:build windows

package libnetwork

import (
	"github.com/docker/docker/libnetwork/etchosts"
)

// Stub implementations for DNS related functions

func (sb *Sandbox) setupResolutionFiles() error {
	return nil
}

func (sb *Sandbox) restorePath() {}

func (sb *Sandbox) updateHostsFile(ifaceIP []string) error {
	return nil
}

func (sb *Sandbox) deleteHostsEntries(recs []etchosts.Record) {}

func (sb *Sandbox) updateDNS(ipv6Enabled bool) error {
	return nil
}
