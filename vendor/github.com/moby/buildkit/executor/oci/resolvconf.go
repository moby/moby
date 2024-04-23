package oci

import (
	"context"
	"os"
	"path/filepath"

	"github.com/docker/docker/libnetwork/resolvconf"
	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/flightcontrol"
	"github.com/pkg/errors"
)

var g flightcontrol.Group[struct{}]
var notFirstRun bool
var lastNotEmpty bool

// overridden by tests
var resolvconfPath = resolvconf.Path

type DNSConfig struct {
	Nameservers   []string
	Options       []string
	SearchDomains []string
}

func GetResolvConf(ctx context.Context, stateDir string, idmap *idtools.IdentityMapping, dns *DNSConfig, netMode pb.NetMode) (string, error) {
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
					return struct{}{}, err
				}
				generate = true
			}
			if !generate {
				fiMain, err := os.Stat(resolvconfPath())
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

		dt, err := os.ReadFile(resolvconfPath())
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return struct{}{}, err
		}

		tmpPath := p + ".tmp"
		if dns != nil {
			var (
				dnsNameservers   = dns.Nameservers
				dnsSearchDomains = dns.SearchDomains
				dnsOptions       = dns.Options
			)
			if len(dns.Nameservers) == 0 {
				dnsNameservers = resolvconf.GetNameservers(dt, resolvconf.IP)
			}
			if len(dns.SearchDomains) == 0 {
				dnsSearchDomains = resolvconf.GetSearchDomains(dt)
			}
			if len(dns.Options) == 0 {
				dnsOptions = resolvconf.GetOptions(dt)
			}

			f, err := resolvconf.Build(tmpPath, dnsNameservers, dnsSearchDomains, dnsOptions)
			if err != nil {
				return struct{}{}, err
			}
			dt = f.Content
		}

		if netMode != pb.NetMode_HOST || len(resolvconf.GetNameservers(dt, resolvconf.IP)) == 0 {
			f, err := resolvconf.FilterResolvDNS(dt, true)
			if err != nil {
				return struct{}{}, err
			}
			dt = f.Content
		}

		if err := os.WriteFile(tmpPath, dt, 0644); err != nil {
			return struct{}{}, err
		}

		if idmap != nil {
			root := idmap.RootPair()
			if err := os.Chown(tmpPath, root.UID, root.GID); err != nil {
				return struct{}{}, err
			}
		}

		if err := os.Rename(tmpPath, p); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	if err != nil {
		return "", err
	}
	return p, nil
}
