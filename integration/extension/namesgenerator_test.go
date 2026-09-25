package extension

import (
	"runtime"
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	namesgeneratorimage "github.com/moby/moby/v2/internal/namesgenerator/image"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"github.com/moby/moby/v2/internal/testutil/fixtures/load"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestExternalNameGenerator(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "the extension binary must be on the daemon's host")
	skip.If(t, testEnv.DaemonInfo.OSType != "linux", "the test fixture uses a Linux image")

	ctx := testutil.StartSpan(baseContext, t)
	extDir := buildExtension(ctx, t, namesgeneratorimage.ID, "./testdata/namesgenerator/cmd/namesgenerator")
	startArgs := []string{"--extension-dir", extDir}
	if testEnv.DaemonInfo.OSType == "linux" {
		startArgs = append(startArgs, "--iptables=false", "--ip6tables=false")
	}

	d := daemon.New(t)
	d.Start(t, startArgs...)
	defer func() {
		if runtime.GOOS == "windows" {
			assert.NilError(t, d.Kill())
			return
		}
		d.Stop(t)
	}()

	engine := d.NewClientT(t)
	assert.NilError(t, load.FrozenImagesLinux(ctx, engine, "busybox:latest"))
	created, err := engine.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     &containertypes.Config{Image: "busybox:latest"},
		HostConfig: &containertypes.HostConfig{},
	})
	assert.NilError(t, err)

	inspect, err := engine.ContainerInspect(ctx, created.ID, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Equal(t, inspect.Container.Name, "/busybox-"+created.ID[:4])

	d.SwarmInit(ctx, t, swarm.InitRequest{AdvertiseAddr: "127.0.0.1"})
	service, err := engine.ServiceCreate(ctx, client.ServiceCreateOptions{
		Spec: swarm.ServiceSpec{
			TaskTemplate: swarm.TaskSpec{
				ContainerSpec: &swarm.ContainerSpec{Image: "busybox:latest"},
			},
		},
	})
	assert.NilError(t, err)

	serviceInspect, err := engine.ServiceInspect(ctx, service.ID, client.ServiceInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Regexp(`^busybox-[0-9a-f]{8}$`, serviceInspect.Service.Spec.Name))
}
