package oci

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"

	"github.com/moby/buildkit/executor/oci/internal/resolvconf"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/flightcontrol"
	"github.com/moby/sys/user"
	"github.com/pkg/errors"
)

var (
	g            flightcontrol.Group[struct{}]
	notFirstRun  bool
	lastNotEmpty bool
)

// overridden by tests
var resolvconfPath = func(netMode pb.NetMode) string {
	// The implementation of resolvconf.Path checks if systemd resolved is activated and chooses the internal
	// resolv.conf (/run/systemd/resolve/resolv.conf) in such a case - see resolvconf_path.go of libnetwork.
	// This, however, can be problematic, see https://github.com/moby/buildkit/issues/2404 and is not necessary
	// in case the networking mode is set to host since the locally (127.0.0.53) running resolved daemon is
	// accessible from inside a host networked container.
	// For details of the implementation see https://github.com/moby/buildkit/pull/5207#discussion_r1705362230.
	if netMode == pb.NetMode_HOST {
		return "/etc/resolv.conf"
	}
	return resolvconf.Path()
}

type DNSConfig struct {
	Nameservers   []string
	Options       []string
	SearchDomains []string
}

func GetResolvConf(ctx context.Context, stateDir string, idmap *user.IdentityMapping, dns *DNSConfig, netMode pb.NetMode) (string, error) {
	p := filepath.Join(stateDir, "resolv.conf")
	if netMode == pb.NetMode_HOST {
		p = filepath.Join(stateDir, "resolv-host.conf")
	}

	_, err := g.Do(ctx, p, func(ctx context.Context) (struct{}, error) {
		generate := !notFirstRun
		notFirstRun = true

		if !generate {
			fi, err := os.Stat(p)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return struct{}{}, errors.WithStack(err)
				}
				generate = true
			}
			if !generate {
				fiMain, err := os.Stat(resolvconfPath(netMode))
				if err != nil {
					if !errors.Is(err, os.ErrNotExist) {
						return struct{}{}, err
					}
					if lastNotEmpty {
						generate = true
						lastNotEmpty = false
					}
				} else if fi.ModTime().Before(fiMain.ModTime()) {
					generate = true
				}
			}
		}

		if !generate {
			return struct{}{}, nil
		}

		rc, err := resolvconf.Load(resolvconfPath(netMode))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return struct{}{}, errors.WithStack(err)
		}

		if dns != nil {
			if len(dns.Nameservers) > 0 {
				var ns []netip.Addr
				for _, addr := range dns.Nameservers {
					ipAddr, err := netip.ParseAddr(addr)
					if err != nil {
						return struct{}{}, errors.WithStack(errors.Wrap(err, "bad nameserver address"))
					}
					ns = append(ns, ipAddr)
				}
				rc.OverrideNameServers(ns)
			}
			if len(dns.SearchDomains) > 0 {
				rc.OverrideSearch(dns.SearchDomains)
			}
			if len(dns.Options) > 0 {
				rc.OverrideOptions(dns.Options)
			}
		}

		if netMode != pb.NetMode_HOST || len(rc.NameServers()) == 0 {
			rc.TransformForLegacyNw(true)
		}

		tmpPath := p + ".tmp"

		dt, err := rc.Generate(false)
		if err != nil {
			return struct{}{}, errors.WithStack(err)
		}

		if err := os.WriteFile(tmpPath, dt, 0644); err != nil {
			return struct{}{}, errors.WithStack(err)
		}

		if idmap != nil {
			uid, gid := idmap.RootPair()
			if err := os.Chown(tmpPath, uid, gid); err != nil {
				return struct{}{}, errors.WithStack(err)
			}
		}

		// TODO(thaJeztah): can we avoid the write -> chown -> rename?
		if err := os.Rename(tmpPath, p); err != nil {
			return struct{}{}, errors.WithStack(err)
		}
		return struct{}{}, nil
	})
	if err != nil {
		return "", err
	}
	return p, nil
}
