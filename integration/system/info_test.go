package system // import "github.com/docker/docker/integration/system"

import (
	"fmt"
	"sort"
	"testing"

	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestInfoAPI(t *testing.T) {
	ctx := setupTest(t)
	client := testEnv.APIClient()

	info, err := client.Info(ctx)
	assert.NilError(t, err)

	// TODO(thaJeztah): make sure we have other tests that run a local daemon and check other fields based on known state.
	assert.Check(t, info.ID != "")
	assert.Check(t, is.Equal(info.Containers, info.ContainersRunning+info.ContainersPaused+info.ContainersStopped))
	assert.Check(t, info.LoggingDriver != "")
	assert.Check(t, info.OperatingSystem != "")
	assert.Check(t, info.NCPU != 0)
	assert.Check(t, info.OSType != "")
	assert.Check(t, info.Architecture != "")
	assert.Check(t, info.MemTotal != 0)
	assert.Check(t, info.KernelVersion != "")
	assert.Check(t, info.Driver != "")
	assert.Check(t, info.ServerVersion != "")
	assert.Check(t, info.SystemTime != "")
	if testEnv.DaemonInfo.OSType != "windows" {
		// Windows currently doesn't have security-options in the info response.
		assert.Check(t, len(info.SecurityOptions) != 0)
	}
}

// TestInfoAPIWarnings verifies that the daemon returns a warning when
// exposing the API on an insecure connection.
//
// This test is slow, because the daemon adds a 15-second delay when
// configured with an insecure connection.
func TestInfoAPIWarnings(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME")

	ctx := testutil.StartSpan(baseContext, t)

	// daemon adds a 15-second delay (cmd/dockerd/loadListeners()) when
	// configured with an insecure connection, so run in parallel.
	// t.Parallel()

	const detectWarning = `WARNING: API is accessible on`
	const expectedWarning = `WARNING: API is accessible on http://%s without encryption.
         Access to the remote API is equivalent to root access on the host. Refer
         to the 'Docker daemon attack surface' section in the documentation for
         more information: https://docs.docker.com/go/attack-surface/`

	// TODO(thaJeztah): add IPv6 (loopback-)addresses (tcp://[::]:1234, tcp://[::3]:2345), but IPv6 is disabled by default inside the container ("/proc/sys/net/ipv6/conf/all/disable_ipv6")
	tests := []struct {
		name            string
		daemonArgs      []string
		expectedWarning string
	}{
		{
			name: "default should not warn",
		},
		{
			name:            "insecure on 127.0.0.1:23750 should warn",
			daemonArgs:      []string{"--host", "tcp://127.0.0.1:23750", "--iptables=false"}, // Make sure each test uses a unique port, to allow running in parallel!
			expectedWarning: fmt.Sprintf(expectedWarning, "127.0.0.1:23750"),
		},
		{
			name:            "insecure on 0.0.0.0:23752 should warn",
			daemonArgs:      []string{"--host", "tcp://0.0.0.0:23752", "--iptables=false"}, // Make sure each test uses a unique port, to allow running in parallel!
			expectedWarning: fmt.Sprintf(expectedWarning, "0.0.0.0:23752"),
		},
		{
			name:            "insecure on 0.0.0.0:23754 with explicit TLS disabled",
			daemonArgs:      []string{"--host", "tcp://0.0.0.0:23754", "--tls=false", "--iptables=false"}, // Make sure each test uses a unique port, to allow running in parallel!
			expectedWarning: fmt.Sprintf(expectedWarning, "0.0.0.0:23754"),
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			d := daemon.New(t)
			c := d.NewClientT(t)
			d.Start(t, tc.daemonArgs...)
			defer d.Stop(t)
			info, err := c.Info(ctx)
			assert.NilError(t, err)
			if tc.expectedWarning == "" {
				for _, w := range info.Warnings {
					if is.Contains(w, detectWarning)().Success() {
						t.Errorf("should not contain, but did: %+v", w)
					}
				}
			} else {
				assert.Check(t, is.Contains(info.Warnings, tc.expectedWarning))
			}
		})
	}
}

func TestInfoDebug(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "FIXME: test starts daemon with -H unix://.....")

	_ = testutil.StartSpan(baseContext, t)

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
