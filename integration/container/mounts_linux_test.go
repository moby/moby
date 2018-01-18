package container

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/integration/util/request"
)

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
