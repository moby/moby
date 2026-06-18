package cniprovider

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	netns "github.com/containernetworking/plugins/pkg/ns"
	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/moby/buildkit/util/bklog"
	"github.com/pkg/errors"
)

func (ns *cniNS) sample() (*resourcestypes.NetworkSample, error) {
	dirfd, err := syscall.Open(filepath.Join("/sys/class/net", ns.vethName, "statistics"), syscall.O_RDONLY, 0)
	if err != nil {
		if errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ENOTDIR) {
			return nil, nil
		}
		return nil, err
	}
	defer syscall.Close(dirfd)

	buf := make([]byte, 32)
	stat := &resourcestypes.NetworkSample{}

	for _, name := range []string{"tx_bytes", "rx_bytes", "tx_packets", "rx_packets", "tx_errors", "rx_errors", "tx_dropped", "rx_dropped"} {
		n, err := readFileAt(dirfd, name, buf)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read %s", name)
		}
		switch name {
		case "tx_bytes":
			stat.TxBytes = n
		case "rx_bytes":
			stat.RxBytes = n
		case "tx_packets":
			stat.TxPackets = n
		case "rx_packets":
			stat.RxPackets = n
		case "tx_errors":
			stat.TxErrors = n
		case "rx_errors":
			stat.RxErrors = n
		case "tx_dropped":
			stat.TxDropped = n
		case "rx_dropped":
			stat.RxDropped = n
		}
	}
	ns.prevSample = stat
	return stat, nil
}

func readFileAt(dirfd int, filename string, buf []byte) (int64, error) {
	fd, err := syscall.Openat(dirfd, filename, syscall.O_RDONLY, 0)
	if err != nil {
		return 0, err
	}
	defer syscall.Close(fd)

	n, err := syscall.Read(fd, buf)
	if err != nil {
		return 0, err
	}
	nn, err := strconv.ParseInt(strings.TrimSpace(string(buf[:n])), 10, 64)
	if err != nil {
		return 0, err
	}
	return nn, nil
}

// withDetachedNetNSIfAny executes fn in $ROOTLESSKIT_STATE_DIR/netns if it exists.
// Otherwise it executes fn in the current netns.
//
// $ROOTLESSKIT_STATE_DIR/netns exists when we are running with RootlessKit >= 2.0 with --detach-netns.
// Since we are left in the host netns, we have to join the "detached" netns that is associated with slirp
// to create CNI namespaces inside it.
// https://github.com/rootless-containers/rootlesskit/pull/379
// https://github.com/containerd/nerdctl/pull/2723
func withDetachedNetNSIfAny(ctx context.Context, fn func(context.Context) error) error {
	if stateDir := os.Getenv("ROOTLESSKIT_STATE_DIR"); stateDir != "" {
		root, err := os.OpenRoot(stateDir)
		if err != nil {
			return err
		}
		defer root.Close()
		if _, err := root.Lstat("netns"); err == nil {
			detachedNetNS := filepath.Join(stateDir, "netns")
			return netns.WithNetNSPath(detachedNetNS, func(_ netns.NetNS) error {
				ctx := context.WithValue(ctx, contextKeyDetachedNetNS, detachedNetNS)
				bklog.G(ctx).Debugf("Entering RootlessKit's detached netns %q", detachedNetNS)
				err2 := fn(ctx)
				bklog.G(ctx).WithError(err2).Debugf("Leaving RootlessKit's detached netns %q", detachedNetNS)
				return err2
			})
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return fn(ctx)
}

func (ns *cniNS) DialContext(ctx context.Context, networkName, address string) (net.Conn, error) {
	// WithNetNSPath only pins the calling thread to the namespace, while
	// net.Dialer runs happy-eyeballs connect attempts and cgo DNS lookups on
	// separate goroutines that would create their sockets in the host
	// namespace. A negative FallbackDelay keeps connects serial on the pinned
	// thread, and the Go resolver with a custom Dial re-enters the namespace
	// for DNS sockets created on resolver goroutines.
	resolverNS, err := netns.GetCurrentNS()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer resolverNS.Close()
	return ns.dialInNS(ctx, networkName, address, ns.dialer(resolverNS))
}

func (ns *cniNS) dialer(resolverNS netns.NetNS) *net.Dialer {
	return &net.Dialer{
		FallbackDelay: -1,
		Resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, networkName, address string) (net.Conn, error) {
				d := &net.Dialer{FallbackDelay: -1}
				if isLoopbackHost(address) {
					// loopback resolvers (systemd-resolved, Docker embedded
					// DNS) are only reachable in the namespace that called
					// DialContext.
					return dialInNetNS(ctx, resolverNS, networkName, address, d)
				}
				return ns.dialInNS(ctx, networkName, address, d)
			},
		},
	}
}

func isLoopbackHost(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (ns *cniNS) dialInNS(ctx context.Context, networkName, address string, dialer *net.Dialer) (net.Conn, error) {
	targetNS, err := netns.GetNS(ns.nativeID)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer targetNS.Close()
	return dialInNetNS(ctx, targetNS, networkName, address, dialer)
}

func dialInNetNS(ctx context.Context, targetNS netns.NetNS, networkName, address string, dialer *net.Dialer) (net.Conn, error) {
	var conn net.Conn
	err := targetNS.Do(func(_ netns.NetNS) error {
		var err error
		conn, err = dialer.DialContext(ctx, networkName, address)
		return err
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return conn, nil
}
