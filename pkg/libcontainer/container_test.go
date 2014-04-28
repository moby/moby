package libcontainer

import (
	"encoding/json"
	"os"
	"testing"
)

func TestContainerJsonFormat(t *testing.T) {
	f, err := os.Open("container.json")
	if err != nil {
		t.Fatal("Unable to open container.json")
	}
	defer f.Close()

	var container *Container
	if err := json.NewDecoder(f).Decode(&container); err != nil {
		t.Fatal("failed to decode container config")
	}
	if container.Hostname != "koye" {
		t.Log("hostname is not set")
		t.Fail()
	}

	if !container.Tty {
		t.Log("tty should be set to true")
		t.Fail()
	}

	if !container.Namespaces.Contains("NEWNET") {
		t.Log("namespaces should contain NEWNET")
		t.Fail()
	}

	if container.Namespaces.Contains("NEWUSER") {
		t.Log("namespaces should not contain NEWUSER")
		t.Fail()
	}

	if !container.CapabilitiesMask.Contains("SYS_ADMIN") {
		t.Log("capabilities mask should contain SYS_ADMIN")
		t.Fail()
	}

	if container.CapabilitiesMask.Get("SYS_ADMIN").Enabled {
		t.Log("SYS_ADMIN should not be enabled in capabilities mask")
		t.Fail()
	}

	if !container.CapabilitiesMask.Get("MKNOD").Enabled {
		t.Log("MKNOD should be enabled in capabilities mask")
		t.Fail()
	}

	if container.CapabilitiesMask.Contains("SYS_CHROOT") {
		t.Log("capabilities mask should not contain SYS_CHROOT")
		t.Fail()
	}
}
