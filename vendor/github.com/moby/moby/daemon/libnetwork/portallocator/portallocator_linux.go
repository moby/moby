package portallocator

import (
	"bufio"
	"fmt"
	"os"
)

func getDynamicPortRange() (start int, end int, _ error) {
	const portRangeKernelParam = "/proc/sys/net/ipv4/ip_local_port_range"
	file, err := os.Open(portRangeKernelParam)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	n, err := fmt.Fscanf(bufio.NewReader(file), "%d\t%d", &start, &end)
	if n != 2 || err != nil {
		if err == nil {
			err = fmt.Errorf("unexpected count of parsed numbers (%d)", n)
		}
		return 0, 0, fmt.Errorf("port allocator - failed to parse system ephemeral port range from %s: %v", portRangeKernelParam, err)
	}
	return start, end, nil
}
