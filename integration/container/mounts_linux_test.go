package container

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/docker/docker/pkg/stdcopy"
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
				Type:        mount.TypeBind,
				Source:      d.Root,
				Target:      "/testdaemonroot",
				BindOptions: &mount.BindOptions{Propagation: mount.PropagationRPrivate}},
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
