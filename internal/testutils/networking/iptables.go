package networking

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/testutil/daemon"
	"golang.org/x/net/context"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/poll"
)

func FirewalldRunning() bool {
	state, err := exec.Command("firewall-cmd", "--state").CombinedOutput()
	return err == nil && strings.TrimSpace(string(state)) == "running"
}

func extractLogTime(s string) (time.Time, error) {
	// time="2025-07-15T13:46:13.414214418Z" level=info msg=""
	re := regexp.MustCompile(`time="([^"]+)"`)
	matches := re.FindStringSubmatch(s)
	if len(matches) < 2 {
		return time.Time{}, fmt.Errorf("timestamp not found in log line: %s, matches: %+v", s, matches)
	}

	return time.Parse(time.RFC3339Nano, matches[1])
}

// FirewalldReload reloads firewalld and waits for the daemon to re-create its rules.
// It's a no-op if firewalld is not running, and the test fails if the reload does
// not complete.
func FirewalldReload(t *testing.T, d *daemon.Daemon) {
	t.Helper()
	if !FirewalldRunning() {
		return
	}
	timeBeforeReload := time.Now()
	res := icmd.RunCommand("firewall-cmd", "--reload")
	assert.NilError(t, res.Error)

	ctx := context.Background()
	poll.WaitOn(t, d.PollCheckLogs(ctx, func(s string) bool {
		if !strings.Contains(s, "Firewalld reload completed") {
			return false
		}
		lastReload, err := extractLogTime(s)
		if err != nil {
			return false
		}
		if lastReload.After(timeBeforeReload) {
			return true
		}
		return false
	}))
}
