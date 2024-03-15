//go:build !windows

package libnetwork

import (
	"context"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/log"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/libnetwork/etchosts"
	"github.com/docker/docker/libnetwork/internal/resolvconf"
	"github.com/docker/docker/libnetwork/types"
	"github.com/pkg/errors"
)

const (
	defaultPrefix = "/var/lib/docker/network/files"
	dirPerm       = 0o755
	filePerm      = 0o644

	resolverIPSandbox = "127.0.0.11"
)

// finishInitDNS is to be called after the container namespace has been created,
// before it the user process is started. The container's support for IPv6 can be
// determined at this point.
func (sb *Sandbox) finishInitDNS() error {
	if err := sb.buildHostsFile(); err != nil {
		return errdefs.System(err)
	}
	for _, ep := range sb.Endpoints() {
		if err := sb.updateHostsFile(ep.getEtcHostsAddrs()); err != nil {
			return errdefs.System(err)
		}
	}
	return nil
}

func (sb *Sandbox) startResolver(restore bool) {
	sb.resolverOnce.Do(func() {
		var err error
		// The resolver is started with proxyDNS=false if the sandbox does not currently
		// have a gateway. So, if the Sandbox is only connected to an 'internal' network,
		// it will not forward DNS requests to external resolvers. The resolver's
		// proxyDNS setting is then updated as network Endpoints are added/removed.
		sb.resolver = NewResolver(resolverIPSandbox, sb.hasExternalAccess(), sb)
		defer func() {
			if err != nil {
				sb.resolver = nil
			}
		}()

		// In the case of live restore container is already running with
		// right resolv.conf contents created before. Just update the
		// external DNS servers from the restored sandbox for embedded
		// server to use.
		if !restore {
			err = sb.rebuildDNS()
			if err != nil {
				log.G(context.TODO()).Errorf("Updating resolv.conf failed for container %s, %q", sb.ContainerID(), err)
				return
			}
		}
		sb.resolver.SetExtServers(sb.extDNS)

		if err = sb.osSbox.InvokeFunc(sb.resolver.SetupFunc(0)); err != nil {
			log.G(context.TODO()).Errorf("Resolver Setup function failed for container %s, %q", sb.ContainerID(), err)
			return
		}

		if err = sb.resolver.Start(); err != nil {
			log.G(context.TODO()).Errorf("Resolver Start failed for container %s, %q", sb.ContainerID(), err)
		}
	})
}

func (sb *Sandbox) setupResolutionFiles() error {
	// Create a hosts file that can be mounted during container setup. For most
	// networking modes (not host networking) it will be re-created before the
	// container start, once its support for IPv6 is known.
	if sb.config.hostsPath == "" {
		sb.config.hostsPath = defaultPrefix + "/" + sb.id + "/hosts"
	}
	dir, _ := filepath.Split(sb.config.hostsPath)
	if err := createBasePath(dir); err != nil {
		return err
	}
	if err := sb.buildHostsFile(); err != nil {
		return err
	}

	return sb.setupDNS()
}

func (sb *Sandbox) buildHostsFile() error {
	sb.restoreHostsPath()

	dir, _ := filepath.Split(sb.config.hostsPath)
	if err := createBasePath(dir); err != nil {
		return err
	}

	// This is for the host mode networking
	if sb.config.useDefaultSandBox && len(sb.config.extraHosts) == 0 {
		// We are working under the assumption that the origin file option had been properly expressed by the upper layer
		// if not here we are going to error out
		if err := copyFile(sb.config.originHostsPath, sb.config.hostsPath); err != nil && !os.IsNotExist(err) {
			return types.InternalErrorf("could not copy source hosts file %s to %s: %v", sb.config.originHostsPath, sb.config.hostsPath, err)
		}
		return nil
	}

	extraContent := make([]etchosts.Record, 0, len(sb.config.extraHosts))
	for _, extraHost := range sb.config.extraHosts {
		extraContent = append(extraContent, etchosts.Record{Hosts: extraHost.name, IP: extraHost.IP})
	}

	// Assume IPv6 support, unless it's definitely disabled.
	buildf := etchosts.Build
	if en, ok := sb.ipv6Enabled(); ok && !en {
		buildf = etchosts.BuildNoIPv6
	}
	if err := buildf(sb.config.hostsPath, extraContent); err != nil {
		return err
	}

	return sb.updateParentHosts()
}

func (sb *Sandbox) updateHostsFile(ifaceIPs []string) error {
	if len(ifaceIPs) == 0 {
		return nil
	}

	if sb.config.originHostsPath != "" {
		return nil
	}

	// User might have provided a FQDN in hostname or split it across hostname
	// and domainname.  We want the FQDN and the bare hostname.
	fqdn := sb.config.hostName
	if sb.config.domainName != "" {
		fqdn += "." + sb.config.domainName
	}
	hosts := fqdn

	if hostName, _, ok := strings.Cut(fqdn, "."); ok {
		hosts += " " + hostName
	}

	var extraContent []etchosts.Record
	for _, ip := range ifaceIPs {
		extraContent = append(extraContent, etchosts.Record{Hosts: hosts, IP: ip})
	}

	sb.addHostsEntries(extraContent)
	return nil
}

func (sb *Sandbox) addHostsEntries(recs []etchosts.Record) {
	// Assume IPv6 support, unless it's definitely disabled.
	if en, ok := sb.ipv6Enabled(); ok && !en {
		var filtered []etchosts.Record
		for _, rec := range recs {
			if addr, err := netip.ParseAddr(rec.IP); err == nil && !addr.Is6() {
				filtered = append(filtered, rec)
			}
		}
		recs = filtered
	}
	if err := etchosts.Add(sb.config.hostsPath, recs); err != nil {
		log.G(context.TODO()).Warnf("Failed adding service host entries to the running container: %v", err)
	}
}

func (sb *Sandbox) deleteHostsEntries(recs []etchosts.Record) {
	if err := etchosts.Delete(sb.config.hostsPath, recs); err != nil {
		log.G(context.TODO()).Warnf("Failed deleting service host entries to the running container: %v", err)
	}
}

func (sb *Sandbox) updateParentHosts() error {
	var pSb *Sandbox

	for _, update := range sb.config.parentUpdates {
		// TODO(thaJeztah): was it intentional for this loop to re-use prior results of pSB? If not, we should make pSb local and always replace here.
		if s, _ := sb.controller.GetSandbox(update.cid); s != nil {
			pSb = s
		}
		if pSb == nil {
			continue
		}
		// TODO(robmry) - filter out IPv6 addresses here if !sb.ipv6Enabled() but...
		// - this is part of the implementation of '--link', which will be removed along
		//   with the rest of legacy networking.
		// - IPv6 addresses shouldn't be allocated if IPv6 is not available in a container,
		//   and that change will come along later.
		// - I think this may be dead code, it's not possible to start a parent container with
		//   '--link child' unless the child has already started ("Error response from daemon:
		//   Cannot link to a non running container"). So, when the child starts and this method
		//   is called with updates for parents, the parents aren't running and GetSandbox()
		//   returns nil.)
		if err := etchosts.Update(pSb.config.hostsPath, update.ip, update.name); err != nil {
			return err
		}
	}

	return nil
}

func (sb *Sandbox) restoreResolvConfPath() {
	if sb.config.resolvConfPath == "" {
		sb.config.resolvConfPath = defaultPrefix + "/" + sb.id + "/resolv.conf"
	}
	sb.config.resolvConfHashFile = sb.config.resolvConfPath + ".hash"
}

func (sb *Sandbox) restoreHostsPath() {
	if sb.config.hostsPath == "" {
		sb.config.hostsPath = defaultPrefix + "/" + sb.id + "/hosts"
	}
}

func (sb *Sandbox) setExternalResolvers(entries []resolvconf.ExtDNSEntry) {
	sb.extDNS = make([]extDNSEntry, 0, len(entries))
	for _, entry := range entries {
		sb.extDNS = append(sb.extDNS, extDNSEntry{
			IPStr:        entry.Addr.String(),
			HostLoopback: entry.HostLoopback,
		})
	}
}

func (c *containerConfig) getOriginResolvConfPath() string {
	if c.originResolvConfPath != "" {
		return c.originResolvConfPath
	}
	// Fallback if not specified.
	return resolvconf.Path()
}

// loadResolvConf reads the resolv.conf file at path, and merges in overrides for
// nameservers, options, and search domains.
func (sb *Sandbox) loadResolvConf(path string) (*resolvconf.ResolvConf, error) {
	rc, err := resolvconf.Load(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	// Proceed with rc, which might be zero-valued if path does not exist.

	rc.SetHeader(`# Generated by Docker Engine.
# This file can be edited; Docker Engine will not make further changes once it
# has been modified.`)
	if len(sb.config.dnsList) > 0 {
		var dnsAddrs []netip.Addr
		for _, ns := range sb.config.dnsList {
			addr, err := netip.ParseAddr(ns)
			if err != nil {
				return nil, errors.Wrapf(err, "bad nameserver address %s", ns)
			}
			dnsAddrs = append(dnsAddrs, addr)
		}
		rc.OverrideNameServers(dnsAddrs)
	}
	if len(sb.config.dnsSearchList) > 0 {
		rc.OverrideSearch(sb.config.dnsSearchList)
	}
	if len(sb.config.dnsOptionsList) > 0 {
		rc.OverrideOptions(sb.config.dnsOptionsList)
	}
	return &rc, nil
}

// For a new sandbox, write an initial version of the container's resolv.conf. It'll
// be a copy of the host's file, with overrides for nameservers, options and search
// domains applied.
func (sb *Sandbox) setupDNS() error {
	// Make sure the directory exists.
	sb.restoreResolvConfPath()
	dir, _ := filepath.Split(sb.config.resolvConfPath)
	if err := createBasePath(dir); err != nil {
		return err
	}

	rc, err := sb.loadResolvConf(sb.config.getOriginResolvConfPath())
	if err != nil {
		return err
	}
	return rc.WriteFile(sb.config.resolvConfPath, sb.config.resolvConfHashFile, filePerm)
}

// Called when an endpoint has joined the sandbox.
func (sb *Sandbox) updateDNS(ipv6Enabled bool) error {
	if mod, err := resolvconf.UserModified(sb.config.resolvConfPath, sb.config.resolvConfHashFile); err != nil || mod {
		return err
	}

	// Load the host's resolv.conf as a starting point.
	rc, err := sb.loadResolvConf(sb.config.getOriginResolvConfPath())
	if err != nil {
		return err
	}
	// For host-networking, no further change is needed.
	if !sb.config.useDefaultSandBox {
		// The legacy bridge network has no internal nameserver. So, strip localhost
		// nameservers from the host's config, then add default nameservers if there
		// are none remaining.
		rc.TransformForLegacyNw(ipv6Enabled)
	}
	return rc.WriteFile(sb.config.resolvConfPath, sb.config.resolvConfHashFile, filePerm)
}

// Embedded DNS server has to be enabled for this sandbox. Rebuild the container's resolv.conf.
func (sb *Sandbox) rebuildDNS() error {
	// Don't touch the file if the user has modified it.
	if mod, err := resolvconf.UserModified(sb.config.resolvConfPath, sb.config.resolvConfHashFile); err != nil || mod {
		return err
	}

	// Load the host's resolv.conf as a starting point.
	rc, err := sb.loadResolvConf(sb.config.getOriginResolvConfPath())
	if err != nil {
		return err
	}

	// Check for IPv6 endpoints in this sandbox. If there are any, and the container has
	// IPv6 enabled, upstream requests from the internal DNS resolver can be made from
	// the container's namespace.
	// TODO(robmry) - this can only check networks connected when the resolver is set up,
	//  the configuration won't be updated if the container gets an IPv6 address later.
	ipv6 := false
	for _, ep := range sb.endpoints {
		if ep.network.enableIPv6 {
			if en, ok := sb.ipv6Enabled(); ok {
				ipv6 = en
			}
			break
		}
	}

	intNS := sb.resolver.NameServer()
	if !intNS.IsValid() {
		return fmt.Errorf("no listen-address for internal resolver")
	}

	// Work out whether ndots has been set from host config or overrides.
	_, sb.ndotsSet = rc.Option("ndots")
	// Swap nameservers for the internal one, and make sure the required options are set.
	var extNameServers []resolvconf.ExtDNSEntry
	extNameServers, err = rc.TransformForIntNS(ipv6, intNS, sb.resolver.ResolverOptions())
	if err != nil {
		return err
	}
	// Extract the list of nameservers that just got swapped out, and store them as
	// upstream nameservers.
	sb.setExternalResolvers(extNameServers)

	// Write the file for the container - preserving old behaviour, not updating the
	// hash file (so, no further updates will be made).
	// TODO(robmry) - I think that's probably accidental, I can't find a reason for it,
	//  and the old resolvconf.Build() function wrote the file but not the hash, which
	//  is surprising. But, before fixing it, a guard/flag needs to be added to
	//  sb.updateDNS() to make sure that when an endpoint joins a sandbox that already
	//  has an internal resolver, the container's resolv.conf is still (re)configured
	//  for an internal resolver.
	return rc.WriteFile(sb.config.resolvConfPath, "", filePerm)
}

func createBasePath(dir string) error {
	return os.MkdirAll(dir, dirPerm)
}

func copyFile(src, dst string) error {
	sBytes, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, sBytes, filePerm)
}
