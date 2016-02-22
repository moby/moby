// +build !windows

package daemon

import (
	"io/ioutil"
	"testing"
)

func TestGetConflictFreeConfigurationWithSerializedBridgeConfig(t *testing.T) {
	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}

	configFile := f.Name()
	f.Write([]byte(`{
		"ipv6": true,
		"iptables": true
}`))
	f.Close()

	c, err := getConflictFreeConfiguration(configFile, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !c.EnableIPv6 {
		t.Fatalf("expected IPv6 to be enabled, got disabled")
	}
	if !c.EnableIPTables {
		t.Fatalf("expected IPTables to be enabled, got disabled")
	}
}
