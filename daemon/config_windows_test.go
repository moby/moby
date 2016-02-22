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
		"bridge": "win-virtual-switch"
}`))
	f.Close()

	c, err := getConflictFreeConfiguration(configFile, nil)
	if err != nil {
		t.Fatal(err)
	}

	if c.VirtualSwitchName != "win-virtual-switch" {
		t.Fatalf("expected virtual switch `win-virtual-switch`, got %s\n", c.VirtualSwitchName)
	}
}
