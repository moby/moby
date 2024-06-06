//go:build windows

package libnetwork

import (
	"github.com/docker/docker/libnetwork/etchosts"
)

// Stub implementations for DNS related functions

func (sb *Sandbox) setupResolutionFiles() error {
	return nil
}

func (sb *Sandbox) restoreHostsPath() {}

func (sb *Sandbox) restoreResolvConfPath() {}

func (sb *Sandbox) updateHostsFile(ifaceIP []string) error {
	return nil
}

func (sb *Sandbox) deleteHostsEntries(recs []etchosts.Record) {}
