//go:build linux

package bridge

import (
	"bytes"
	"os"
	"testing"
)

func TestSetupIPForwarding(t *testing.T) {
	// Read current setting and ensure the original value gets restored
	procSetting := readCurrentIPForwardingSetting(t)
	defer reconcileIPForwardingSetting(t, procSetting)

	// Disable IP Forwarding if enabled
	if bytes.Equal(procSetting, []byte("1\n")) {
		writeIPForwardingSetting(t, []byte{'0', '\n'})
	}

	// Set IP Forwarding
	if err := setupIPForwarding(true, true); err != nil {
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
	err := os.WriteFile(ipv4ForwardConf, chars, ipv4ForwardConfPerm)
	if err != nil {
		t.Fatalf("Can't execute or cleanup after test: Failed to reset IP forwarding: %v", err)
	}
}

func reconcileIPForwardingSetting(t *testing.T, original []byte) {
	current := readCurrentIPForwardingSetting(t)
	if !bytes.Equal(original, current) {
		writeIPForwardingSetting(t, original)
	}
}
