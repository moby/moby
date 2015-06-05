package fs

import (
	"strings"
	"testing"

	"github.com/docker/libcontainer/configs"
)

var (
	prioMap = []*configs.IfPrioMap{
		{
			Interface: "test",
			Priority:  5,
		},
	}
)

func TestNetPrioSetIfPrio(t *testing.T) {
	helper := NewCgroupTestUtil("net_prio", t)
	defer helper.cleanup()

	helper.CgroupData.c.NetPrioIfpriomap = prioMap
	netPrio := &NetPrioGroup{}
	if err := netPrio.Set(helper.CgroupPath, helper.CgroupData.c); err != nil {
		t.Fatal(err)
	}

	value, err := getCgroupParamString(helper.CgroupPath, "net_prio.ifpriomap")
	if err != nil {
		t.Fatalf("Failed to parse net_prio.ifpriomap - %s", err)
	}
	if !strings.Contains(value, "test 5") {
		t.Fatal("Got the wrong value, set net_prio.ifpriomap failed.")
	}
}
