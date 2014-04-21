package configuration

import (
	"github.com/dotcloud/docker/daemon/execdriver/native/template"
	"testing"
)

func TestSetReadonlyRootFs(t *testing.T) {
	var (
		container = template.New()
		opts      = []string{
			"fs.readonly=true",
		}
	)

	if container.ReadonlyFs {
		t.Fatal("container should not have a readonly rootfs by default")
	}
	if err := ParseConfiguration(container, nil, opts); err != nil {
		t.Fatal(err)
	}

	if !container.ReadonlyFs {
		t.Fatal("container should have a readonly rootfs")
	}
}

func TestConfigurationsDoNotConflict(t *testing.T) {
	var (
		container1 = template.New()
		container2 = template.New()
		opts       = []string{
			"cap.add=NET_ADMIN",
		}
	)

	if err := ParseConfiguration(container1, nil, opts); err != nil {
		t.Fatal(err)
	}

	if !container1.CapabilitiesMask.Get("NET_ADMIN").Enabled {
		t.Fatal("container one should have NET_ADMIN enabled")
	}
	if container2.CapabilitiesMask.Get("NET_ADMIN").Enabled {
		t.Fatal("container two should not have NET_ADMIN enabled")
	}
}

func TestCpusetCpus(t *testing.T) {
	var (
		container = template.New()
		opts      = []string{
			"cgroups.cpuset.cpus=1,2",
		}
	)
	if err := ParseConfiguration(container, nil, opts); err != nil {
		t.Fatal(err)
	}

	if expected := "1,2"; container.Cgroups.CpusetCpus != expected {
		t.Fatalf("expected %s got %s for cpuset.cpus", expected, container.Cgroups.CpusetCpus)
	}
}

func TestAppArmorProfile(t *testing.T) {
	var (
		container = template.New()
		opts      = []string{
			"apparmor_profile=koye-the-protector",
		}
	)
	if err := ParseConfiguration(container, nil, opts); err != nil {
		t.Fatal(err)
	}
	if expected := "koye-the-protector"; container.Context["apparmor_profile"] != expected {
		t.Fatalf("expected profile %s got %s", expected, container.Context["apparmor_profile"])
	}
}

func TestCpuShares(t *testing.T) {
	var (
		container = template.New()
		opts      = []string{
			"cgroups.cpu_shares=1048",
		}
	)
	if err := ParseConfiguration(container, nil, opts); err != nil {
		t.Fatal(err)
	}

	if expected := int64(1048); container.Cgroups.CpuShares != expected {
		t.Fatalf("expected cpu shares %d got %d", expected, container.Cgroups.CpuShares)
	}
}

func TestCgroupMemory(t *testing.T) {
	var (
		container = template.New()
		opts      = []string{
			"cgroups.memory=500m",
		}
	)
	if err := ParseConfiguration(container, nil, opts); err != nil {
		t.Fatal(err)
	}

	if expected := int64(500 * 1024 * 1024); container.Cgroups.Memory != expected {
		t.Fatalf("expected memory %d got %d", expected, container.Cgroups.Memory)
	}
}

func TestAddCap(t *testing.T) {
	var (
		container = template.New()
		opts      = []string{
			"cap.add=MKNOD",
			"cap.add=SYS_ADMIN",
		}
	)
	if err := ParseConfiguration(container, nil, opts); err != nil {
		t.Fatal(err)
	}

	if !container.CapabilitiesMask.Get("MKNOD").Enabled {
		t.Fatal("container should have MKNOD enabled")
	}
	if !container.CapabilitiesMask.Get("SYS_ADMIN").Enabled {
		t.Fatal("container should have SYS_ADMIN enabled")
	}
}

func TestDropCap(t *testing.T) {
	var (
		container = template.New()
		opts      = []string{
			"cap.drop=MKNOD",
		}
	)
	// enabled all caps like in privileged mode
	for _, c := range container.CapabilitiesMask {
		c.Enabled = true
	}
	if err := ParseConfiguration(container, nil, opts); err != nil {
		t.Fatal(err)
	}

	if container.CapabilitiesMask.Get("MKNOD").Enabled {
		t.Fatal("container should not have MKNOD enabled")
	}
}

func TestDropNamespace(t *testing.T) {
	var (
		container = template.New()
		opts      = []string{
			"ns.drop=NEWNET",
		}
	)
	if err := ParseConfiguration(container, nil, opts); err != nil {
		t.Fatal(err)
	}

	if container.Namespaces.Get("NEWNET").Enabled {
		t.Fatal("container should not have NEWNET enabled")
	}
}
