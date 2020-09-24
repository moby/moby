package portallocator

import (
	"bytes"
	"fmt"
	"os/exec"
)

func getDynamicPortRange() (start int, end int, err error) {
	portRangeKernelSysctl := []string{"net.inet.ip.portrange.hifirst", "net.ip.portrange.hilast"}
	portRangeLowCmd := exec.Command("/sbin/sysctl", portRangeKernelSysctl[0])
	var portRangeLowOut bytes.Buffer
	portRangeLowCmd.Stdout = &portRangeLowOut
	cmdErr := portRangeLowCmd.Run()
	if cmdErr != nil {
		return 0, 0, fmt.Errorf("port allocator - sysctl net.inet.ip.portrange.hifirst failed: %v", err)
	}
	n, err := fmt.Sscanf(portRangeLowOut.String(), "%d", &start)
	if n != 1 || err != nil {
		if err == nil {
			err = fmt.Errorf("unexpected count of parsed numbers (%d)", n)
		}
		return 0, 0, fmt.Errorf("port allocator - failed to parse system ephemeral port range start from %s: %v", portRangeLowOut.String(), err)
	}

	portRangeHighCmd := exec.Command("/sbin/sysctl", portRangeKernelSysctl[1])
	var portRangeHighOut bytes.Buffer
	portRangeHighCmd.Stdout = &portRangeHighOut
	cmdErr = portRangeHighCmd.Run()
	if cmdErr != nil {
		return 0, 0, fmt.Errorf("port allocator - sysctl net.inet.ip.portrange.hilast failed: %v", err)
	}
	n, err = fmt.Sscanf(portRangeHighOut.String(), "%d", &end)
	if n != 1 || err != nil {
		if err == nil {
			err = fmt.Errorf("unexpected count of parsed numbers (%d)", n)
		}
		return 0, 0, fmt.Errorf("port allocator - failed to parse system ephemeral port range end from %s: %v", portRangeHighOut.String(), err)
	}
	return start, end, nil
}
