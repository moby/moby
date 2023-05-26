package system // import "github.com/docker/docker/integration/system"

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestInfoAPI(t *testing.T) {
	defer setupTest(t)()
	client := testEnv.APIClient()

	info, err := client.Info(context.Background())
	assert.NilError(t, err)

	// always shown fields
	stringsToCheck := []string{
		"ID",
		"Containers",
		"ContainersRunning",
		"ContainersPaused",
		"ContainersStopped",
		"Images",
		"LoggingDriver",
		"OperatingSystem",
		"NCPU",
		"OSType",
		"Architecture",
		"MemTotal",
		"KernelVersion",
		"Driver",
		"ServerVersion",
		"SecurityOptions"}

	out := fmt.Sprintf("%+v", info)
	for _, linePrefix := range stringsToCheck {
		assert.Check(t, is.Contains(out, linePrefix))
	}
}

func TestInfoAPIWarnings(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME")
	d := daemon.New(t)
	c := d.NewClientT(t)

	d.Start(t, "-H=0.0.0.0:23756", "-H="+d.Sock())
	defer d.Stop(t)

	info, err := c.Info(context.Background())
	assert.NilError(t, err)

	stringsToCheck := []string{
		"Access to the remote API is equivalent to root access",
		"http://0.0.0.0:23756",
	}

	out := fmt.Sprintf("%+v", info)
	for _, linePrefix := range stringsToCheck {
		assert.Check(t, is.Contains(out, linePrefix))
	}
}

func TestInfoDebug(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME: test starts daemon with -H unix://.....")

	d := daemon.New(t)
	d.Start(t, "--debug")
	defer d.Stop(t)

	info := d.Info(t)
	assert.Equal(t, info.Debug, true)

	// Note that the information below is not tied to debug-mode being enabled.
	assert.Check(t, info.NFd != 0)

	// TODO need a stable way to generate event listeners
	// assert.Check(t, info.NEventsListener != 0)
	assert.Check(t, info.NGoroutines != 0)
	assert.Check(t, info.SystemTime != "")
	assert.Equal(t, info.DockerRootDir, d.Root)
}

func TestInfoInsecureRegistries(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME: test starts daemon with -H unix://.....")

	const (
		registryCIDR = "192.168.1.0/24"
		registryHost = "insecurehost.com:5000"
	)

	d := daemon.New(t)
	d.Start(t, "--insecure-registry="+registryCIDR, "--insecure-registry="+registryHost)
	defer d.Stop(t)

	info := d.Info(t)
	assert.Assert(t, is.Len(info.RegistryConfig.InsecureRegistryCIDRs, 2))
	cidrs := []string{
		info.RegistryConfig.InsecureRegistryCIDRs[0].String(),
		info.RegistryConfig.InsecureRegistryCIDRs[1].String(),
	}
	assert.Assert(t, is.Contains(cidrs, registryCIDR))
	assert.Assert(t, is.Contains(cidrs, "127.0.0.0/8"))
	assert.DeepEqual(t, *info.RegistryConfig.IndexConfigs["docker.io"], registry.IndexInfo{Name: "docker.io", Mirrors: []string{}, Secure: true, Official: true})
	assert.DeepEqual(t, *info.RegistryConfig.IndexConfigs[registryHost], registry.IndexInfo{Name: registryHost, Mirrors: []string{}, Secure: false, Official: false})
}

func TestInfoListeners(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME: test starts daemon with -H unix://.....")

	d := daemon.New(t)
	d.Start(t, "-H=127.0.0.1:5732", "-H=[::1]:5733")
	defer d.Stop(t)

	info := d.Info(t)
	expected := []system.ListenerInfo{
		{Address: d.Sock(), Insecure: false},
		{Address: "http://127.0.0.1:5732", Insecure: true},
		{Address: "http://[::1]:5733", Insecure: true},
	}
	assert.DeepEqual(t, info.Listeners, expected)
}

func TestInfoRegistryMirrors(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME: test starts daemon with -H unix://.....")

	const (
		registryMirror1 = "https://192.168.1.2"
		registryMirror2 = "http://registry-mirror.example.com:5000"
	)

	d := daemon.New(t)
	d.Start(t, "--registry-mirror="+registryMirror1, "--registry-mirror="+registryMirror2)
	defer d.Stop(t)

	info := d.Info(t)
	sort.Strings(info.RegistryConfig.Mirrors)
	assert.DeepEqual(t, info.RegistryConfig.Mirrors, []string{registryMirror2 + "/", registryMirror1 + "/"})
}
