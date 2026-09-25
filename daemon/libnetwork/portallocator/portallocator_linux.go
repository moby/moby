package portallocator

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
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

// getReservedPorts returns the ports the kernel excludes from automatic
// assignment, read from /proc/sys/net/ipv4/ip_local_reserved_ports
// (e.g. "8080,9148" or "30000-32767"). Reserved ports outside the
// begin-end allocation range are dropped, the allocator never picks
// them anyway.
func getReservedPorts(begin, end int) (map[uint16]struct{}, error) {
	const reservedPortsKernelParam = "/proc/sys/net/ipv4/ip_local_reserved_ports"
	data, err := os.ReadFile(reservedPortsKernelParam)
	if err != nil {
		return nil, err
	}
	reserved, err := parseReservedPorts(string(data), begin, end)
	if err != nil {
		return nil, fmt.Errorf("port allocator - failed to parse system reserved ports from %s: %v", reservedPortsKernelParam, err)
	}
	return reserved, nil
}

// parseReservedPorts parses a port list in the kernel format, a
// comma-separated list of single ports ("1080") and port ranges
// ("30000-32767"), keeping only ports within begin-end.
func parseReservedPorts(list string, begin, end int) (map[uint16]struct{}, error) {
	list = strings.TrimSpace(list)
	if list == "" {
		return nil, nil
	}
	reserved := map[uint16]struct{}{}
	for entry := range strings.SplitSeq(list, ",") {
		first, last, isRange := strings.Cut(entry, "-")
		entryBegin, err := strconv.ParseUint(first, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid port %q: %v", entry, err)
		}
		entryEnd := entryBegin
		if isRange {
			entryEnd, err = strconv.ParseUint(last, 10, 16)
			if err != nil {
				return nil, fmt.Errorf("invalid port range %q: %v", entry, err)
			}
			if entryEnd < entryBegin {
				return nil, fmt.Errorf("invalid port range %q", entry)
			}
		}
		for port := max(int(entryBegin), begin); port <= min(int(entryEnd), end); port++ {
			reserved[uint16(port)] = struct{}{}
		}
	}
	return reserved, nil
}
