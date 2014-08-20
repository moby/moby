package configuration

import (
	"testing"

	"github.com/docker/docker/daemon/execdriver/native/template"
	"github.com/docker/libcontainer/security/capabilities"
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

func TestSetReadonlyRootFs(t *testing.T) {
	var (
		container = template.New()
		opts      = []string{
			"fs.readonly=true",
		}
	)

	if container.MountConfig.ReadonlyFs {
		t.Fatal("container should not have a readonly rootfs by default")
	}
	if err := ParseConfiguration(container, nil, opts); err != nil {
		t.Fatal(err)
	}

	if !container.MountConfig.ReadonlyFs {
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

	if !hasCapability("NET_ADMIN", container1.Capabilities) {
		t.Fatal("container one should have NET_ADMIN enabled")
	}
	if hasCapability("NET_ADMIN", container2.Capabilities) {
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

	if expected := "koye-the-protector"; container.AppArmorProfile != expected {
		t.Fatalf("expected profile %s got %s", expected, container.AppArmorProfile)
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

func TestMemory(t *testing.T) {
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

func TestMemoryReservation(t *testing.T) {
	var (
		container = template.New()
		opts      = []string{
			"cgroups.memory_reservation=500m",
		}
	)
	if err := ParseConfiguration(container, nil, opts); err != nil {
		t.Fatal(err)
	}

	if expected := int64(500 * 1024 * 1024); container.Cgroups.MemoryReservation != expected {
		t.Fatalf("expected memory reservation %d got %d", expected, container.Cgroups.MemoryReservation)
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

	if !hasCapability("MKNOD", container.Capabilities) {
		t.Fatal("container should have MKNOD enabled")
	}
	if !hasCapability("SYS_ADMIN", container.Capabilities) {
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
	container.Capabilities = capabilities.GetAllCapabilities()
	if err := ParseConfiguration(container, nil, opts); err != nil {
		t.Fatal(err)
	}

	if hasCapability("MKNOD", container.Capabilities) {
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

	if container.Namespaces["NEWNET"] {
		t.Fatal("container should not have NEWNET enabled")
	}
}
