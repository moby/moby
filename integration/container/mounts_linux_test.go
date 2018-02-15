package container // import "github.com/docker/docker/integration/container"

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/system"
	"github.com/gotestyourself/gotestyourself/fs"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerShmNoLeak(t *testing.T) {
	t.Parallel()
	d := daemon.New(t, "docker", "dockerd", daemon.Config{})
	client, err := d.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	d.StartWithBusybox(t)
	defer d.Stop(t)

	ctx := context.Background()
	cfg := container.Config{
		Image: "busybox",
		Cmd:   []string{"top"},
	}

	ctr, err := client.ContainerCreate(ctx, &cfg, nil, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	defer client.ContainerRemove(ctx, ctr.ID, types.ContainerRemoveOptions{Force: true})

	if err := client.ContainerStart(ctx, ctr.ID, types.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}

	// this should recursively bind mount everything in the test daemons root
	// except of course we are hoping that the previous containers /dev/shm mount did not leak into this new container
	hc := container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: d.Root,
				Target: "/testdaemonroot",
			},
		},
	}
	cfg.Cmd = []string{"/bin/sh", "-c", fmt.Sprintf("mount | grep testdaemonroot | grep containers | grep %s", ctr.ID)}
	cfg.AttachStdout = true
	cfg.AttachStderr = true
	ctrLeak, err := client.ContainerCreate(ctx, &cfg, &hc, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	attach, err := client.ContainerAttach(ctx, ctrLeak.ID, types.ContainerAttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := client.ContainerStart(ctx, ctrLeak.ID, types.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}

	buf := bytes.NewBuffer(nil)

	if _, err := stdcopy.StdCopy(buf, buf, attach.Reader); err != nil {
		t.Fatal(err)
	}

	out := bytes.TrimSpace(buf.Bytes())
	if !bytes.Equal(out, []byte{}) {
		t.Fatalf("mount leaked: %s", string(out))
	}
}

func TestContainerNetworkMountsNoChown(t *testing.T) {
	// chown only applies to Linux bind mounted volumes; must be same host to verify
	skip.If(t, testEnv.DaemonInfo.OSType != "linux" || !testEnv.IsLocalDaemon())

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
	require.NoError(t, err)
	defer cli.Close()

	ctrCreate, err := cli.ContainerCreate(ctx, &config, &hostConfig, &network.NetworkingConfig{}, "")
	require.NoError(t, err)
	// container will exit immediately because of no tty, but we only need the start sequence to test the condition
	err = cli.ContainerStart(ctx, ctrCreate.ID, types.ContainerStartOptions{})
	require.NoError(t, err)

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
	require.NoError(t, err)
	assert.Equal(t, uint32(0), statT.UID(), "bind mounted network file should not change ownership from root")
}

func TestMountDaemonRoot(t *testing.T) {
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
