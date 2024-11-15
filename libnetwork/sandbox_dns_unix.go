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
	"go.opentelemetry.io/otel"
)

const (
	defaultPrefix = "/var/lib/docker/network/files"
	dirPerm       = 0o755
	filePerm      = 0o644

	resolverIPSandbox = "127.0.0.11"
)

// AddHostsEntry adds an entry to /etc/hosts.
func (sb *Sandbox) AddHostsEntry(ctx context.Context, name, ip string) error {
	sb.config.extraHosts = append(sb.config.extraHosts, extraHost{name: name, IP: ip})
	return sb.rebuildHostsFile(ctx)
}

// UpdateHostsEntry updates the IP address in a /etc/hosts entry where the
// name matches the regular expression regexp.
func (sb *Sandbox) UpdateHostsEntry(regexp, ip string) error {
	return etchosts.Update(sb.config.hostsPath, ip, regexp)
}

// rebuildHostsFile builds the container's /etc/hosts file, based on the current
// state of the Sandbox (including extra hosts). If called after the container
// namespace has been created, before the user process is started, the container's
// support for IPv6 can be determined and IPv6 hosts will be included/excluded
// accordingly.
func (sb *Sandbox) rebuildHostsFile(ctx context.Context) error {
	var ifaceIPs []netip.Addr
	for _, ep := range sb.Endpoints() {
		ifaceIPs = append(ifaceIPs, ep.getEtcHostsAddrs()...)
	}
	if err := sb.buildHostsFile(ctx, ifaceIPs); err != nil {
		return errdefs.System(err)
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

func (sb *Sandbox) setupResolutionFiles(ctx context.Context) error {
	_, span := otel.Tracer("").Start(ctx, "libnetwork.Sandbox.setupResolutionFiles")
	defer span.End()

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
	if err := sb.buildHostsFile(ctx, nil); err != nil {
		return err
	}

	return sb.setupDNS()
}

func (sb *Sandbox) buildHostsFile(ctx context.Context, ifaceIPs []netip.Addr) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.buildHostsFile")
	defer span.End()

	sb.restoreHostsPath()

	dir, _ := filepath.Split(sb.config.hostsPath)
	if err := createBasePath(dir); err != nil {
		return err
	}

	// This is for the host mode networking. If extra hosts are supplied, even though
	// it's host-networking, the container's hosts file is not based on the host's -
	// so that it's possible to override a hostname that's in the host's hosts file.
	// See analysis of how this came about in:
	// https://github.com/moby/moby/pull/48823#issuecomment-2461777129
	if sb.config.useDefaultSandBox && len(sb.config.extraHosts) == 0 {
		// We are working under the assumption that the origin file option had been properly expressed by the upper layer
		// if not here we are going to error out
		if err := copyFile(sb.config.originHostsPath, sb.config.hostsPath); err != nil && !os.IsNotExist(err) {
			return types.InternalErrorf("could not copy source hosts file %s to %s: %v", sb.config.originHostsPath, sb.config.hostsPath, err)
		}
		return nil
	}

	extraContent := make([]etchosts.Record, 0, len(sb.config.extraHosts)+len(ifaceIPs))
	for _, host := range sb.config.extraHosts {
		addr, err := netip.ParseAddr(host.IP)
		if err != nil {
			return errdefs.InvalidParameter(fmt.Errorf("could not parse extra host IP %s: %v", host.IP, err))
		}
		extraContent = append(extraContent, etchosts.Record{Hosts: host.name, IP: addr})
	}
	extraContent = append(extraContent, sb.makeHostsRecs(ifaceIPs)...)

	// Assume IPv6 support, unless it's definitely disabled.
	if en, ok := sb.IPv6Enabled(); ok && !en {
		return etchosts.BuildNoIPv6(sb.config.hostsPath, extraContent)
	}
	return etchosts.Build(sb.config.hostsPath, extraContent)
}

func (sb *Sandbox) makeHostsRecs(ifaceIPs []netip.Addr) []etchosts.Record {
	if len(ifaceIPs) == 0 {
		return nil
	}

	// User might have provided a FQDN in hostname or split it across hostname
	// and domainname.  We want the FQDN and the bare hostname.
	hosts := sb.config.hostName
	if sb.config.domainName != "" {
		hosts += "." + sb.config.domainName
	}

	if hn, _, ok := strings.Cut(hosts, "."); ok {
		hosts += " " + hn
	}

	var recs []etchosts.Record
	for _, ip := range ifaceIPs {
		recs = append(recs, etchosts.Record{Hosts: hosts, IP: ip})
	}
	return recs
}

func (sb *Sandbox) addHostsEntries(ctx context.Context, ifaceAddrs []netip.Addr) {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.addHostsEntries")
	defer span.End()

	// Assume IPv6 support, unless it's definitely disabled.
	if en, ok := sb.IPv6Enabled(); ok && !en {
		var filtered []netip.Addr
		for _, addr := range ifaceAddrs {
			if !addr.Is6() {
				filtered = append(filtered, addr)
			}
		}
		ifaceAddrs = filtered
	}
	if err := etchosts.Add(sb.config.hostsPath, sb.makeHostsRecs(ifaceAddrs)); err != nil {
		log.G(context.TODO()).Warnf("Failed adding service host entries to the running container: %v", err)
	}
}

func (sb *Sandbox) deleteHostsEntries(ifaceAddrs []netip.Addr) {
	if err := etchosts.Delete(sb.config.hostsPath, sb.makeHostsRecs(ifaceAddrs)); err != nil {
		log.G(context.TODO()).Warnf("Failed deleting service host entries to the running container: %v", err)
	}
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

	intNS := sb.resolver.NameServer()
	if !intNS.IsValid() {
		return fmt.Errorf("no listen-address for internal resolver")
	}

	// Work out whether ndots has been set from host config or overrides.
	_, sb.ndotsSet = rc.Option("ndots")
	// Swap nameservers for the internal one, and make sure the required options are set.
	var extNameServers []resolvconf.ExtDNSEntry
	extNameServers, err = rc.TransformForIntNS(intNS, sb.resolver.ResolverOptions())
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
