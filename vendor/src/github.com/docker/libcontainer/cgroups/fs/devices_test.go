package fs

import (
	"testing"

	"github.com/docker/libcontainer/configs"
)

var (
	allowedDevices = []*configs.Device{
		{
			Path:        "/dev/zero",
			Type:        'c',
			Major:       1,
			Minor:       5,
			Permissions: "rwm",
			FileMode:    0666,
		},
	}
	allowedList = "c 1:5 rwm"
)

func TestDevicesSetAllow(t *testing.T) {
	helper := NewCgroupTestUtil("devices", t)
	defer helper.cleanup()

	helper.writeFileContents(map[string]string{
		"device.deny": "a",
	})

	helper.CgroupData.c.AllowAllDevices = false
	helper.CgroupData.c.AllowedDevices = allowedDevices
	devices := &DevicesGroup{}
	if err := devices.Set(helper.CgroupPath, helper.CgroupData.c); err != nil {
		t.Fatal(err)
	}

	// FIXME: this doesn't make sence, the file devices.allow under real cgroupfs
	// is not allowed to read. Our test path don't have cgroupfs mounted.
	value, err := getCgroupParamString(helper.CgroupPath, "devices.allow")
	if err != nil {
		t.Fatalf("Failed to parse devices.allow - %s", err)
	}

	if value != allowedList {
		t.Fatal("Got the wrong value, set devices.allow failed.")
	}
}
