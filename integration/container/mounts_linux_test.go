package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/internal/test/request"
	"github.com/docker/docker/pkg/system"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/fs"
	"gotest.tools/skip"
)

func TestContainerNetworkMountsNoChown(t *testing.T) {
	// chown only applies to Linux bind mounted volumes; must be same host to verify
	skip.If(t, testEnv.DaemonInfo.OSType != "linux" || testEnv.IsRemoteDaemon())

	defer setupTest(t)()

	ctx := context.Background()

	tmpDir := fs.NewDir(t, "network-file-mounts", fs.WithMode(0755), fs.WithFile("nwfile", "network file bind mount", fs.WithMode(0644)))
	defer tmpDir.Remove()

	tmpNWFileMount := tmpDir.Join("nwfile")

	config := container.Config{
		Image: "busybox",
	}
	hostConfig := container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   "bind",
				Source: tmpNWFileMount,
				Target: "/etc/resolv.conf",
			},
			{
				Type:   "bind",
				Source: tmpNWFileMount,
				Target: "/etc/hostname",
			},
			{
				Type:   "bind",
				Source: tmpNWFileMount,
				Target: "/etc/hosts",
			},
		},
	}

	cli, err := client.NewEnvClient()
	assert.NilError(t, err)
	defer cli.Close()

	ctrCreate, err := cli.ContainerCreate(ctx, &config, &hostConfig, &network.NetworkingConfig{}, "")
	assert.NilError(t, err)
	// container will exit immediately because of no tty, but we only need the start sequence to test the condition
	err = cli.ContainerStart(ctx, ctrCreate.ID, types.ContainerStartOptions{})
	assert.NilError(t, err)

	// Check that host-located bind mount network file did not change ownership when the container was started
	// Note: If the user specifies a mountpath from the host, we should not be
	// attempting to chown files outside the daemon's metadata directory
	// (represented by `daemon.repository` at init time).
	// This forces users who want to use user namespaces to handle the
	// ownership needs of any external files mounted as network files
	// (/etc/resolv.conf, /etc/hosts, /etc/hostname) separately from the
	// daemon. In all other volume/bind mount situations we have taken this
	// same line--we don't chown host file content.
	// See GitHub PR 34224 for details.
	statT, err := system.Stat(tmpNWFileMount)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(uint32(0), statT.UID()), "bind mounted network file should not change ownership from root")
}

func TestMountDaemonRoot(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux" || testEnv.IsRemoteDaemon())
	t.Parallel()

	client := request.NewAPIClient(t)
	ctx := context.Background()
	info, err := client.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		desc        string
		propagation mount.Propagation
		expected    mount.Propagation
	}{
		{
			desc:        "default",
			propagation: "",
			expected:    mount.PropagationRSlave,
		},
		{
			desc:        "private",
			propagation: mount.PropagationPrivate,
		},
		{
			desc:        "rprivate",
			propagation: mount.PropagationRPrivate,
		},
		{
			desc:        "slave",
			propagation: mount.PropagationSlave,
		},
		{
			desc:        "rslave",
			propagation: mount.PropagationRSlave,
			expected:    mount.PropagationRSlave,
		},
		{
			desc:        "shared",
			propagation: mount.PropagationShared,
		},
		{
			desc:        "rshared",
			propagation: mount.PropagationRShared,
			expected:    mount.PropagationRShared,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			test := test
			t.Parallel()

			propagationSpec := fmt.Sprintf(":%s", test.propagation)
			if test.propagation == "" {
				propagationSpec = ""
			}
			bindSpecRoot := info.DockerRootDir + ":" + "/foo" + propagationSpec
			bindSpecSub := filepath.Join(info.DockerRootDir, "containers") + ":/foo" + propagationSpec

			for name, hc := range map[string]*container.HostConfig{
				"bind root":    {Binds: []string{bindSpecRoot}},
				"bind subpath": {Binds: []string{bindSpecSub}},
				"mount root": {
					Mounts: []mount.Mount{
						{
							Type:        mount.TypeBind,
							Source:      info.DockerRootDir,
							Target:      "/foo",
							BindOptions: &mount.BindOptions{Propagation: test.propagation},
						},
					},
				},
				"mount subpath": {
					Mounts: []mount.Mount{
						{
							Type:        mount.TypeBind,
							Source:      filepath.Join(info.DockerRootDir, "containers"),
							Target:      "/foo",
							BindOptions: &mount.BindOptions{Propagation: test.propagation},
						},
					},
				},
			} {
				t.Run(name, func(t *testing.T) {
					hc := hc
					t.Parallel()

					c, err := client.ContainerCreate(ctx, &container.Config{
						Image: "busybox",
						Cmd:   []string{"true"},
					}, hc, nil, "")

					if err != nil {
						if test.expected != "" {
							t.Fatal(err)
						}
						// expected an error, so this is ok and should not continue
						return
					}
					if test.expected == "" {
						t.Fatal("expected create to fail")
					}

					defer func() {
						if err := client.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
							panic(err)
						}
					}()

					inspect, err := client.ContainerInspect(ctx, c.ID)
					if err != nil {
						t.Fatal(err)
					}
					if len(inspect.Mounts) != 1 {
						t.Fatalf("unexpected number of mounts: %+v", inspect.Mounts)
					}

					m := inspect.Mounts[0]
					if m.Propagation != test.expected {
						t.Fatalf("got unexpected propagation mode, expected %q, got: %v", test.expected, m.Propagation)
					}
				})
			}
		})
	}
}
