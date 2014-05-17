package libcontainer

import (
	"encoding/json"
	"os"
	"testing"
)

// Checks whether the expected capability is specified in the capabilities.
func hasCapability(expected string, capabilities []string) bool {
	for _, capability := range capabilities {
		if capability == expected {
			return true
		}
	}
	return false
}

func TestContainerJsonFormat(t *testing.T) {
	f, err := os.Open("container.json")
	if err != nil {
		t.Fatal("Unable to open container.json")
	}
	defer f.Close()

	var container *Container
	if err := json.NewDecoder(f).Decode(&container); err != nil {
		t.Fatalf("failed to decode container config: %s", err)
	}
	if container.Hostname != "koye" {
		t.Log("hostname is not set")
		t.Fail()
	}

	if !container.Tty {
		t.Log("tty should be set to true")
		t.Fail()
	}

	if !container.Namespaces["NEWNET"] {
		t.Log("namespaces should contain NEWNET")
		t.Fail()
	}

	if container.Namespaces["NEWUSER"] {
		t.Log("namespaces should not contain NEWUSER")
		t.Fail()
	}

	if hasCapability("SYS_ADMIN", container.Capabilities) {
		t.Log("SYS_ADMIN should not be enabled in capabilities mask")
		t.Fail()
	}

	if !hasCapability("MKNOD", container.Capabilities) {
		t.Log("MKNOD should be enabled in capabilities mask")
		t.Fail()
	}

	if hasCapability("SYS_CHROOT", container.Capabilities) {
		t.Log("capabilities mask should not contain SYS_CHROOT")
		t.Fail()
	}
}
