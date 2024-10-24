//go:build linux

package bridge

import (
	"bytes"
	"os"
	"testing"

	"github.com/docker/docker/internal/testutils/netnsutils"
)

func TestSetupIPForwarding(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()

	// Read current setting
	procSetting := readCurrentIPForwardingSetting(t)

	// Disable IP Forwarding if enabled
	if bytes.Equal(procSetting, []byte("1\n")) {
		writeIPForwardingSetting(t, []byte{'0', '\n'})
	}

	// Set IP Forwarding
	if err := setupIPv4Forwarding(true); err != nil {
		t.Fatalf("Failed to setup IP forwarding: %v", err)
	}

	// Read new setting
	procSetting = readCurrentIPForwardingSetting(t)
	if !bytes.Equal(procSetting, []byte("1\n")) {
		t.Fatal("Failed to effectively setup IP forwarding")
	}
}

func readCurrentIPForwardingSetting(t *testing.T) []byte {
	procSetting, err := os.ReadFile(ipv4ForwardConf)
	if err != nil {
		t.Fatalf("Can't execute test: Failed to read current IP forwarding setting: %v", err)
	}
	return procSetting
}

func writeIPForwardingSetting(t *testing.T, chars []byte) {
	err := os.WriteFile(ipv4ForwardConf, chars, forwardConfPerm)
	if err != nil {
		t.Fatalf("Can't execute test: Failed to set IP forwarding: %v", err)
	}
}
