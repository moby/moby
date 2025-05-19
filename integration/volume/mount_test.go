package volume

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/build"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/internal/safepath"
	"github.com/docker/docker/testutil/fakecontext"
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

			inspect, err := apiClient.ContainerInspect(ctx, id)
			if assert.Check(t, err) {
				assert.Check(t, is.Equal(inspect.State.ExitCode, 0))
			}

			assert.Check(t, is.Equal(strings.TrimSpace(output.Stderr), ""))
			assert.Check(t, is.Equal(strings.TrimSpace(output.Stdout), tc.expected))
		})
	}
}

func TestRunMountImage(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.48"), "skip test from new feature")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "image mounts not supported on Windows")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	for _, tc := range []struct {
		name         string
		opts         mount.ImageOptions
		cmd          []string
		createErr    string
		startErr     string
		expected     string
		skipPlatform string
	}{
		{name: "image", cmd: []string{"cat", "/image/foo"}, expected: "bar"},
		{name: "image_tag", cmd: []string{"cat", "/image/foo"}, expected: "bar"},

		{name: "subdir", opts: mount.ImageOptions{Subpath: "subdir"}, cmd: []string{"ls", "/image"}, expected: "hello"},
		{name: "subdir link", opts: mount.ImageOptions{Subpath: "hack/good"}, cmd: []string{"ls", "/image"}, expected: "hello"},
		{name: "subdir link outside context", opts: mount.ImageOptions{Subpath: "hack/bad"}, cmd: []string{"ls", "/image"}, startErr: (&safepath.ErrEscapesBase{}).Error()},
		{name: "file", opts: mount.ImageOptions{Subpath: "subdir/hello"}, cmd: []string{"cat", "/image"}, expected: "world"},
		{name: "relative with backtracks", opts: mount.ImageOptions{Subpath: "../../../../../../etc/passwd"}, cmd: []string{"cat", "/image"}, createErr: "subpath must be a relative path within the volume"},
		{name: "not existing", opts: mount.ImageOptions{Subpath: "not-existing-path"}, cmd: []string{"cat", "/image"}, startErr: (&safepath.ErrNotAccessible{}).Error()},

		{name: "image_remove", cmd: []string{"cat", "/image/foo"}, expected: "bar"},
		// Expected is duplicated because the container runs twice
		{name: "image_remove_force", cmd: []string{"cat", "/image/foo"}, expected: "barbar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testImage := setupTestImage(t, ctx, apiClient, tc.name, testEnv.UsingSnapshotter())
			if testImage != "" {
				defer apiClient.ImageRemove(ctx, testImage, image.RemoveOptions{Force: true})
			}

			cfg := containertypes.Config{
				Image: "busybox",
				Cmd:   tc.cmd,
			}

			hostCfg := containertypes.HostConfig{
				Mounts: []mount.Mount{
					{
						Type:         mount.TypeImage,
						Source:       testImage,
						Target:       "/image",
						ImageOptions: &tc.opts,
					},
				},
			}

			startContainer := func(id string) {
				startErr := apiClient.ContainerStart(ctx, id, containertypes.StartOptions{})
				if tc.startErr != "" {
					assert.ErrorContains(t, startErr, tc.startErr)
					return
				}
				assert.NilError(t, startErr)
			}

			ctrName := strings.ReplaceAll(t.Name(), "/", "_")
			create, creatErr := apiClient.ContainerCreate(ctx, &cfg, &hostCfg, &network.NetworkingConfig{}, nil, ctrName)
			id := create.ID
			if id != "" {
				defer container.Remove(ctx, t, apiClient, id, containertypes.RemoveOptions{Force: true})
			}

			if tc.createErr != "" {
				assert.ErrorContains(t, creatErr, tc.createErr)
				return
			}
			assert.NilError(t, creatErr, "container creation failed")
			startContainer(id)

			// Test image mounted is in use logic
			if tc.name == "image_remove" {
				img, _ := apiClient.ImageInspect(ctx, testImage)
				imgId := strings.Split(img.ID, ":")[1]
				_, removeErr := apiClient.ImageRemove(ctx, testImage, image.RemoveOptions{})
				assert.ErrorContains(t, removeErr, fmt.Sprintf(`container %s is using its referenced image %s`, id[:12], imgId[:12]))
			}

			// Test that the container servives a restart when mounted image is removed
			if tc.name == "image_remove_force" {
				stopErr := apiClient.ContainerStop(ctx, id, containertypes.StopOptions{})
				assert.NilError(t, stopErr)

				_, removeErr := apiClient.ImageRemove(ctx, testImage, image.RemoveOptions{Force: true})
				assert.NilError(t, removeErr)

				startContainer(id)
			}

			output, err := container.Output(ctx, apiClient, id)
			assert.Check(t, err)

			inspect, err := apiClient.ContainerInspect(ctx, id)
			if tc.startErr == "" && assert.Check(t, err) {
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
func setupTestVolume(t *testing.T, apiClient client.APIClient) string {
	t.Helper()
	ctx := context.Background()

	volumeName := t.Name() + "-volume"

	err := apiClient.VolumeRemove(ctx, volumeName, true)
	assert.NilError(t, err, "failed to clean volume")

	_, err = apiClient.VolumeCreate(ctx, volume.CreateOptions{
		Name: volumeName,
	})
	assert.NilError(t, err, "failed to setup volume")

	rootFs := "/"
	target := "/volume"
	if testEnv.DaemonInfo.OSType == "windows" {
		rootFs = `C:`
		target = `C:\volume`
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
		container.WithMount(mount.Mount{
			Type:   mount.TypeVolume,
			Source: volumeName,
			Target: target,
		}),
		container.WithCmd("sh", "-c", initCmd+"; ls -lah /volume /volume/hack/"),
	}
	if testEnv.DaemonInfo.OSType == "windows" {
		// Can't create symlinks under HyperV isolation
		opts = append(opts, container.WithIsolation(containertypes.IsolationProcess))
	}

	cid := container.Run(ctx, t, apiClient, opts...)
	defer container.Remove(ctx, t, apiClient, cid, containertypes.RemoveOptions{Force: true})
	output, err := container.Output(ctx, apiClient, cid)

	assert.NilError(t, err)
	assert.Assert(t, is.Equal(output.Stderr, ""))

	inspect, err := apiClient.ContainerInspect(ctx, cid)
	assert.NilError(t, err)
	assert.Assert(t, is.Equal(inspect.State.ExitCode, 0))

	return volumeName
}

func setupTestImage(t *testing.T, ctx context.Context, apiClient client.APIClient, test string, snapshotter bool) string {
	imgName := "test-image"

	if test == "image_tag" {
		imgName += ":foo"
	}

	var symlink string
	if snapshotter {
		symlink = "../../../../rootfs"
	} else {
		symlink = "../../../../../docker"
	}

	//nolint:dupword // ignore "Duplicate words (subdir) found (dupword)"
	dockerfile := `
		FROM busybox as symlink
		RUN mkdir /hack \
			&& ln -s "../subdir" /hack/good \
			&& ln -s "` + symlink + `" /hack/bad
		#--
		FROM scratch
		COPY foo /
		COPY subdir subdir
		COPY --from=symlink /hack /hack
		`

	source := fakecontext.New(
		t,
		"",
		fakecontext.WithDockerfile(dockerfile),
		fakecontext.WithFile("foo", "bar"),
		fakecontext.WithFile("subdir/hello", "world"),
	)
	defer source.Close()

	resp, err := apiClient.ImageBuild(ctx,
		source.AsTarReader(t),
		build.ImageBuildOptions{
			Remove:      false,
			ForceRemove: false,
			Tags:        []string{imgName},
		})
	assert.NilError(t, err)

	out := bytes.NewBuffer(nil)
	_, err = io.Copy(out, resp.Body)
	assert.Check(t, resp.Body.Close())
	assert.NilError(t, err)

	_, err = apiClient.ImageInspect(ctx, imgName)
	if err != nil {
		t.Log(out)
	}
	assert.NilError(t, err)

	return imgName
}
