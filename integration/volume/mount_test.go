package volume

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/safepath"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestRunMountVolumeSubdir(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.45"), "skip test from new feature")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	testVolumeName := setupTestVolume(t, apiClient)

	for _, tc := range []struct {
		name         string
		opts         mount.VolumeOptions
		cmd          []string
		volumeTarget string
		createErr    string
		startErr     string
		expected     string
		skipPlatform string
	}{
		{name: "subdir", opts: mount.VolumeOptions{Subpath: "subdir"}, cmd: []string{"ls", "/volume"}, expected: "hello.txt"},
		{name: "subdir link", opts: mount.VolumeOptions{Subpath: "hack/good"}, cmd: []string{"ls", "/volume"}, expected: "hello.txt"},
		{name: "subdir with copy data", opts: mount.VolumeOptions{Subpath: "bin"}, volumeTarget: "/bin", cmd: []string{"ls", "/bin/busybox"}, expected: "/bin/busybox", skipPlatform: "windows:copy not supported on Windows"},
		{name: "file", opts: mount.VolumeOptions{Subpath: "bar.txt"}, cmd: []string{"cat", "/volume"}, expected: "foo", skipPlatform: "windows:file bind mounts not supported on Windows"},
		{name: "relative with backtracks", opts: mount.VolumeOptions{Subpath: "../../../../../../etc/passwd"}, cmd: []string{"cat", "/volume"}, createErr: "subpath must be a relative path within the volume"},
		{name: "not existing", opts: mount.VolumeOptions{Subpath: "not-existing-path"}, cmd: []string{"cat", "/volume"}, startErr: (&safepath.ErrNotAccessible{}).Error()},

		{name: "mount link", opts: mount.VolumeOptions{Subpath: filepath.Join("hack", "root")}, cmd: []string{"ls", "/volume"}, startErr: (&safepath.ErrEscapesBase{}).Error()},
		{name: "mount link link", opts: mount.VolumeOptions{Subpath: filepath.Join("hack", "bad")}, cmd: []string{"ls", "/volume"}, startErr: (&safepath.ErrEscapesBase{}).Error()}, //nolint:dupword
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipPlatform != "" {
				platform, reason, _ := strings.Cut(tc.skipPlatform, ":")
				if testEnv.DaemonInfo.OSType == platform {
					t.Skip(reason)
				}
			}

			cfg := containertypes.Config{
				Image: "busybox",
				Cmd:   tc.cmd,
			}
			hostCfg := containertypes.HostConfig{
				Mounts: []mount.Mount{
					{
						Type:          mount.TypeVolume,
						Source:        testVolumeName,
						Target:        "/volume",
						VolumeOptions: &tc.opts,
					},
				},
			}
			if testEnv.DaemonInfo.OSType == "windows" {
				hostCfg.Mounts[0].Target = `C:\volume`
			}
			if tc.volumeTarget != "" {
				hostCfg.Mounts[0].Target = tc.volumeTarget
			}

			ctrName := strings.ReplaceAll(t.Name(), "/", "_")
			create, creatErr := apiClient.ContainerCreate(ctx, &cfg, &hostCfg, &network.NetworkingConfig{}, nil, ctrName)
			id := create.ID
			if id != "" {
				defer apiClient.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})
			}

			if tc.createErr != "" {
				assert.ErrorContains(t, creatErr, tc.createErr)
				return
			}
			assert.NilError(t, creatErr, "container creation failed")

			startErr := apiClient.ContainerStart(ctx, id, containertypes.StartOptions{})
			if tc.startErr != "" {
				assert.ErrorContains(t, startErr, tc.startErr)
				return
			}
			assert.NilError(t, startErr)

			output, err := container.Output(ctx, apiClient, id)
			assert.Check(t, err)
			t.Logf("stdout:\n%s", output.Stdout)
			t.Logf("stderr:\n%s", output.Stderr)

			inspect, err := apiClient.ContainerInspect(ctx, id)
			if assert.Check(t, err) {
				assert.Check(t, is.Equal(inspect.State.ExitCode, 0))
			}

			assert.Check(t, is.Equal(strings.TrimSpace(output.Stderr), ""))
			assert.Check(t, is.Equal(strings.TrimSpace(output.Stdout), tc.expected))
		})
	}
}

// setupTestVolume sets up a volume with:
// .
// |-- bar.txt                        (file with "foo")
// |-- bin                            (directory)
// |-- subdir                         (directory)
// |   |-- hello.txt                  (file with "world")
// |-- hack                           (directory)
// |   |-- root                       (symlink to /)
// |   |-- good                       (symlink to ../subdir)
// |   |-- bad                        (symlink to root)
func setupTestVolume(t *testing.T, client client.APIClient) string {
	t.Helper()
	ctx := context.Background()

	volumeName := t.Name() + "-volume"

	err := client.VolumeRemove(ctx, volumeName, true)
	assert.NilError(t, err, "failed to clean volume")

	_, err = client.VolumeCreate(ctx, volume.CreateOptions{
		Name: volumeName,
	})
	assert.NilError(t, err, "failed to setup volume")

	mount := mount.Mount{
		Type:   mount.TypeVolume,
		Source: volumeName,
		Target: "/volume",
	}

	rootFs := "/"
	if testEnv.DaemonInfo.OSType == "windows" {
		mount.Target = `C:\volume`
		rootFs = `C:`
	}

	initCmd := "echo foo > /volume/bar.txt && " +
		"mkdir /volume/bin && " +
		"mkdir /volume/subdir && " +
		"echo world > /volume/subdir/hello.txt && " +
		"mkdir /volume/hack && " +
		"ln -s " + rootFs + " /volume/hack/root && " +
		"ln -s ../subdir /volume/hack/good && " +
		"ln -s root /volume/hack/bad &&" +
		"mkdir /volume/hack/iwanttobehackedwithtoctou"

	opts := []func(*container.TestContainerConfig){
		container.WithMount(mount),
		container.WithCmd("sh", "-c", initCmd+"; ls -lah /volume /volume/hack/"),
	}
	if testEnv.DaemonInfo.OSType == "windows" {
		// Can't create symlinks under HyperV isolation
		opts = append(opts, container.WithIsolation(containertypes.IsolationProcess))
	}

	cid := container.Run(ctx, t, client, opts...)
	defer client.ContainerRemove(ctx, cid, containertypes.RemoveOptions{Force: true})
	output, err := container.Output(ctx, client, cid)

	t.Logf("Setup stderr:\n%s", output.Stderr)
	t.Logf("Setup stdout:\n%s", output.Stdout)

	assert.NilError(t, err)
	assert.Assert(t, is.Equal(output.Stderr, ""))

	inspect, err := client.ContainerInspect(ctx, cid)
	assert.NilError(t, err)
	assert.Assert(t, is.Equal(inspect.State.ExitCode, 0))

	return volumeName
}
