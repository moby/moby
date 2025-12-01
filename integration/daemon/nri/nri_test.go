package nri

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/containerd/nri/pkg/api"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestNRIContainerCreateEnvVarMod(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "cannot start a separate daemon with NRI enabled on Windows")
	skip.If(t, testEnv.IsRootless)

	ctx := testutil.StartSpan(baseContext, t)

	sockPath := filepath.Join(t.TempDir(), "nri.sock")

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t,
		"--nri-opts=enable=true,socket-path="+sockPath,
		"--iptables=false", "--ip6tables=false",
	)
	defer d.Stop(t)
	c := d.NewClientT(t)

	tests := []struct {
		name         string
		ctrCreateAdj *api.ContainerAdjustment
		expEnv       string
	}{
		{
			name:         "env/set",
			ctrCreateAdj: &api.ContainerAdjustment{Env: []*api.KeyValue{{Key: "NRI_SAYS", Value: "hello"}}},
			expEnv:       "NRI_SAYS=hello",
		},
		{
			name:         "env/modify",
			ctrCreateAdj: &api.ContainerAdjustment{Env: []*api.KeyValue{{Key: "HOSTNAME", Value: "nrivictim"}}},
			expEnv:       "HOSTNAME=nrivictim",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stopPlugin := startBuiltinPlugin(ctx, t, builtinPluginConfig{
				pluginName:   "nri-test-plugin",
				pluginIdx:    "00",
				sockPath:     sockPath,
				ctrCreateAdj: tc.ctrCreateAdj,
			})
			defer stopPlugin()

			ctrId := container.Run(ctx, t, c)
			defer func() { _, _ = c.ContainerRemove(ctx, ctrId, client.ContainerRemoveOptions{Force: true}) }()

			inspect, err := c.ContainerInspect(ctx, ctrId, client.ContainerInspectOptions{})
			if assert.Check(t, err) {
				assert.Check(t, is.Contains(inspect.Container.Config.Env, tc.expEnv))
			}
		})
	}
}

func TestNRIContainerCreateUnsupportedAdj(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "cannot start a separate daemon with NRI enabled on Windows")
	skip.If(t, testEnv.IsRootless)

	ctx := testutil.StartSpan(baseContext, t)

	sockPath := filepath.Join(t.TempDir(), "nri.sock")

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t,
		"--nri-opts=enable=true,socket-path="+sockPath,
		"--iptables=false", "--ip6tables=false",
	)
	defer d.Stop(t)
	c := d.NewClientT(t)

	tests := []struct {
		name         string
		ctrCreateAdj *api.ContainerAdjustment
		expErr       string
	}{
		{
			name:         "hooks",
			ctrCreateAdj: &api.ContainerAdjustment{Hooks: &api.Hooks{CreateRuntime: []*api.Hook{{Path: "/bin/true"}}}},
			expErr:       "unsupported container adjustments: hooks",
		},
		{
			name:         "cdi",
			ctrCreateAdj: &api.ContainerAdjustment{CDIDevices: []*api.CDIDevice{{Name: "/dev/somedevice"}}},
			expErr:       "unsupported container adjustments: CDI",
		},
		{
			name: "cpu",
			ctrCreateAdj: &api.ContainerAdjustment{Linux: &api.LinuxContainerAdjustment{Resources: &api.LinuxResources{
				Cpu: &api.LinuxCPU{Shares: &api.OptionalUInt64{Value: 123}},
			}}},
			expErr: "unsupported container adjustments: linux.resources.cpu",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stopPlugin := startBuiltinPlugin(ctx, t, builtinPluginConfig{
				pluginName:   "nri-test-plugin",
				pluginIdx:    "00",
				sockPath:     sockPath,
				ctrCreateAdj: tc.ctrCreateAdj,
			})
			defer stopPlugin()

			res, err := c.ContainerCreate(ctx, client.ContainerCreateOptions{Image: "busybox:latest"})
			if err != nil {
				_, _ = c.ContainerRemove(ctx, res.ID, client.ContainerRemoveOptions{Force: true})
			}
			assert.Check(t, is.ErrorContains(err, tc.expErr))
		})
	}
}

func TestNRIContainerCreateAddMount(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "cannot start a separate daemon with NRI enabled on Windows")
	skip.If(t, testEnv.IsRootless)

	ctx := testutil.StartSpan(baseContext, t)

	sockPath := filepath.Join(t.TempDir(), "nri.sock")

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t,
		"--nri-opts=enable=true,socket-path="+sockPath,
		"--iptables=false", "--ip6tables=false",
	)
	defer d.Stop(t)
	c := d.NewClientT(t)

	// Create and populate a directory for containers to mount.
	dirToMount := t.TempDir()
	if err := os.WriteFile(filepath.Join(dirToMount, "testfile.txt"), []byte("hello\n"), 0o644); err != nil {
		assert.NilError(t, err)
	}
	const (
		mountPoint  = "/mountpoint"
		ctrTestFile = "/mountpoint/testfile.txt"
		exitOk      = 0
		exitFail    = 1
	)

	// Create and populate a volume.
	const volName = "nri-test-volume"
	_, err := c.VolumeCreate(ctx, client.VolumeCreateOptions{Name: volName})
	assert.NilError(t, err)
	defer func() {
		_, _ = c.VolumeRemove(ctx, volName, client.VolumeRemoveOptions{Force: true})
	}()
	// Populate the volume with a test file.
	_ = container.Run(ctx, t, c,
		container.WithAutoRemove,
		container.WithMount(mount.Mount{Type: "volume", Source: volName, Target: mountPoint}),
		container.WithCmd("sh", "-c", "echo hello > "+ctrTestFile),
	)

	tests := []struct {
		name         string
		ctrCreateAdj *api.ContainerAdjustment

		expMountRead  int
		expMountWrite int
	}{
		{
			name: "mount/bind/ro",
			ctrCreateAdj: &api.ContainerAdjustment{Mounts: []*api.Mount{{
				Type:        "bind",
				Source:      dirToMount,
				Destination: mountPoint,
				Options:     []string{"ro"},
			}}},
			expMountRead:  exitOk,
			expMountWrite: exitFail,
		},
		{
			name: "mount/bind/rw",
			ctrCreateAdj: &api.ContainerAdjustment{Mounts: []*api.Mount{{
				Type:        "bind",
				Source:      dirToMount,
				Destination: mountPoint,
			}}},
			expMountRead:  exitOk,
			expMountWrite: exitOk,
		},
		{
			name: "mount/volume/ro",
			ctrCreateAdj: &api.ContainerAdjustment{Mounts: []*api.Mount{{
				Type:        "volume",
				Source:      volName,
				Destination: mountPoint,
				Options:     []string{"ro"},
			}}},
			expMountRead:  exitOk,
			expMountWrite: exitFail,
		},
		{
			name: "mount/volume/rw",
			ctrCreateAdj: &api.ContainerAdjustment{Mounts: []*api.Mount{{
				Type:        "volume",
				Source:      volName,
				Destination: mountPoint,
			}}},
			expMountRead:  exitOk,
			expMountWrite: exitOk,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stopPlugin := startBuiltinPlugin(ctx, t, builtinPluginConfig{
				pluginName:   "nri-test-plugin",
				pluginIdx:    "00",
				sockPath:     sockPath,
				ctrCreateAdj: tc.ctrCreateAdj,
			})
			defer stopPlugin()

			ctrId := container.Run(ctx, t, c)
			defer func() { _, _ = c.ContainerRemove(ctx, ctrId, client.ContainerRemoveOptions{Force: true}) }()

			res, err := container.Exec(ctx, c, ctrId, []string{"cat", ctrTestFile})
			if assert.Check(t, err) {
				assert.Check(t, is.Equal(res.ExitCode, tc.expMountRead))
				assert.Check(t, is.Equal(res.Stdout(), "hello\n"))
			}
			res, err = container.Exec(ctx, c, ctrId, []string{"touch", ctrTestFile})
			if assert.Check(t, err) {
				assert.Check(t, is.Equal(res.ExitCode, tc.expMountWrite))
			}
		})
	}
}
