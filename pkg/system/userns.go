// +build linux

package system

import (
	"bufio"
	"fmt"
	"os"
)

// RunningInUserNS detects whether we are currently running in a user
// namespace.  Copied from github.com/lxc/lxd/shared/util.go
func RunningInUserNS() bool {
	file, err := os.Open("/proc/self/uid_map")
	if err != nil {
		/*
		 * This kernel-provided file only exists if user namespaces are
		 * supported
		 */
		return false
	}
	defer file.Close()

	buf := bufio.NewReader(file)
	l, _, err := buf.ReadLine()
	if err != nil {
		return false
	}

	line := string(l)
	var a, b, c int64
	fmt.Sscanf(line, "%d %d %d", &a, &b, &c)
	/*
	 * We assume we are in the initial user namespace if we have a full
	 * range - 4294967295 uids starting at uid 0.
	 */
	if a == 0 && b == 0 && c == 4294967295 {
		return false
	}
	return true
}
