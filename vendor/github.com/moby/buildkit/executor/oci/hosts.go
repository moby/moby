package oci

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/idtools"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/identity"
	"github.com/pkg/errors"
)

const defaultHostname = "buildkitsandbox"

func GetHostsFile(ctx context.Context, stateDir string, extraHosts []executor.HostIP, idmap *idtools.IdentityMapping, hostname string) (string, func(), error) {
	if len(extraHosts) != 0 || hostname != defaultHostname {
		return makeHostsFile(stateDir, extraHosts, idmap, hostname)
	}

	_, err := g.Do(ctx, stateDir, func(ctx context.Context) (interface{}, error) {
		_, _, err := makeHostsFile(stateDir, nil, idmap, hostname)
		return nil, err
	})
	if err != nil {
		return "", nil, err
	}
	return filepath.Join(stateDir, "hosts"), func() {}, nil
}

func makeHostsFile(stateDir string, extraHosts []executor.HostIP, idmap *idtools.IdentityMapping, hostname string) (string, func(), error) {
	p := filepath.Join(stateDir, "hosts")
	if len(extraHosts) != 0 || hostname != defaultHostname {
		p += "." + identity.NewID()
	}
	_, err := os.Stat(p)
	if err == nil {
		return "", func() {}, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", nil, err
	}

	b := &bytes.Buffer{}
	if _, err := b.Write([]byte(initHostsFile(hostname))); err != nil {
		return "", nil, err
	}

	for _, h := range extraHosts {
		if _, err := b.Write([]byte(fmt.Sprintf("%s\t%s\n", h.IP.String(), h.Host))); err != nil {
			return "", nil, err
		}
	}

	tmpPath := p + ".tmp"
	if err := os.WriteFile(tmpPath, b.Bytes(), 0644); err != nil {
		return "", nil, err
	}

	if idmap != nil {
		root := idmap.RootPair()
		if err := os.Chown(tmpPath, root.UID, root.GID); err != nil {
			return "", nil, err
		}
	}

	if err := os.Rename(tmpPath, p); err != nil {
		return "", nil, err
	}
	return p, func() {
		os.RemoveAll(p)
	}, nil
}

func initHostsFile(hostname string) string {
	var hosts string
	if hostname != "" {
		hosts = fmt.Sprintf("127.0.0.1	localhost %s", hostname)
	} else {
		hosts = fmt.Sprintf("127.0.0.1	localhost %s", defaultHostname)
	}
	hosts = fmt.Sprintf("%s\n::1	localhost ip6-localhost ip6-loopback\n", hosts)
	return hosts
}
