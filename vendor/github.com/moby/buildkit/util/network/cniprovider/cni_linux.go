package cniprovider

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ns"
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

	n, err := syscall.Read(fd, buf[:])
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
		detachedNetNS := filepath.Join(stateDir, "netns")
		if _, err := os.Lstat(detachedNetNS); !errors.Is(err, os.ErrNotExist) {
			return ns.WithNetNSPath(detachedNetNS, func(_ ns.NetNS) error {
				ctx = context.WithValue(ctx, contextKeyDetachedNetNS, detachedNetNS)
				bklog.G(ctx).Debugf("Entering RootlessKit's detached netns %q", detachedNetNS)
				err2 := fn(ctx)
				bklog.G(ctx).WithError(err2).Debugf("Leaving RootlessKit's detached netns %q", detachedNetNS)
				return err2
			})
		}
	}
	return fn(ctx)
}
