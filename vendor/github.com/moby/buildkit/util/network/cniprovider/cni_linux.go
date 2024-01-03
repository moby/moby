package cniprovider

import (
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/moby/buildkit/util/network"
	"github.com/pkg/errors"
)

func (ns *cniNS) sample() (*network.Sample, error) {
	dirfd, err := syscall.Open(filepath.Join("/sys/class/net", ns.vethName, "statistics"), syscall.O_RDONLY, 0)
	if err != nil {
		if errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ENOTDIR) {
			return nil, nil
		}
		return nil, err
	}
	defer syscall.Close(dirfd)

	buf := make([]byte, 32)
	stat := &network.Sample{}

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
