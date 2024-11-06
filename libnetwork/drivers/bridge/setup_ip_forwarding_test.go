//go:build linux

package bridge

import (
	"os"
	"testing"

	"github.com/docker/docker/internal/testutils/netnsutils"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSetupIPForwarding(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	// Disable IP Forwarding if enabled
	_, err := configureIPForwarding(ipv4ForwardConf, '0')
	assert.NilError(t, err)

	// Set IP Forwarding
	err = setupIPv4Forwarding(true)
	assert.NilError(t, err)

	// Read new setting
	procSetting, err := os.ReadFile(ipv4ForwardConf)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(procSetting, []byte{'1', '\n'}))
}
