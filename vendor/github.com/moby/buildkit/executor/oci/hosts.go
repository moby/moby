package oci

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/identity"
	"github.com/moby/sys/user"
	"github.com/pkg/errors"
)

const defaultHostname = "buildkitsandbox"

func GetHostsFile(ctx context.Context, root *os.Root, extraHosts []executor.HostIP, idmap *user.IdentityMapping, hostname string) (string, func(), error) {
	if len(extraHosts) != 0 || hostname != defaultHostname {
		return makeHostsFile(root, extraHosts, idmap, hostname)
	}

	_, err := g.Do(ctx, root.Name(), func(ctx context.Context) (struct{}, error) {
		_, _, err := makeHostsFile(root, nil, idmap, hostname)
		return struct{}{}, err
	})
	if err != nil {
		return "", nil, err
	}
	return "hosts", func() {}, nil
}

func makeHostsFile(root *os.Root, extraHosts []executor.HostIP, idmap *user.IdentityMapping, hostname string) (string, func(), error) {
	name := "hosts"
	if len(extraHosts) != 0 || hostname != defaultHostname {
		name += "." + identity.NewID()
	}
	_, err := root.Stat(name)
	if err == nil {
		return "", func() {}, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", nil, errors.WithStack(err)
	}

	b := &bytes.Buffer{}
	if _, err := b.Write([]byte(initHostsFile(hostname))); err != nil {
		return "", nil, errors.WithStack(err)
	}

	for _, h := range extraHosts {
		if _, err := b.Write(fmt.Appendf(nil, "%s\t%s\n", h.IP.String(), h.Host)); err != nil {
			return "", nil, errors.WithStack(err)
		}
	}

	tmpName := name + ".tmp"
	if err := root.WriteFile(tmpName, b.Bytes(), 0644); err != nil {
		return "", nil, errors.WithStack(err)
	}

	if idmap != nil {
		uid, gid := idmap.RootPair()
		if err := root.Chown(tmpName, uid, gid); err != nil {
			return "", nil, errors.WithStack(err)
		}
	}

	if err := root.Rename(tmpName, name); err != nil {
		return "", nil, errors.WithStack(err)
	}
	cleanRoot, err := root.OpenRoot(".")
	if err != nil {
		return "", nil, errors.WithStack(err)
	}
	return name, func() {
		cleanRoot.RemoveAll(name)
		cleanRoot.Close()
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
