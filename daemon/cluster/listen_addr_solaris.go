package cluster

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

func (c *Cluster) resolveSystemAddr() (net.IP, error) {
	defRouteCmd := "/usr/sbin/ipadm show-addr -p -o addr " +
		"`/usr/sbin/route get default | /usr/bin/grep interface | " +
		"/usr/bin/awk '{print $2}'`"
	out, err := exec.Command("/usr/bin/bash", "-c", defRouteCmd).Output()
	if err != nil {
		return nil, fmt.Errorf("cannot get default route: %v", err)
	}

	defInterface := strings.SplitN(string(out), "/", 2)
	defInterfaceIP := net.ParseIP(defInterface[0])

	return defInterfaceIP, nil
}
