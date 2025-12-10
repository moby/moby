package nri

import (
	"path/filepath"
	"testing"

	"github.com/containerd/nri/pkg/api"
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
