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
		t.Log("failed to decode container config")
		t.FailNow()
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
		t.Log("capabilities should contain SYS_ADMIN")
		t.Fail()
	}

	if container.CapabilitiesMask.Contains("SYS_CHROOT") {
		t.Log("capabitlies should not contain SYS_CHROOT")
		t.Fail()
	}

	if container.Cgroups.CpuShares != 1024 {
		t.Log("cpu shares not set correctly")
		t.Fail()
	}

	if container.Cgroups.Memory != 5248000 {
		t.Log("memory limit not set correctly")
		t.Fail()
	}
}
