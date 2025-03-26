package networking

import (
	"os/exec"
	"strings"
	"testing"
)

func FirewalldRunning() bool {
	state, err := exec.Command("firewall-cmd", "--state").CombinedOutput()
	return err == nil && strings.TrimSpace(string(state)) == "running"
}
