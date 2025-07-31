package system

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/v2/testutil"
	"github.com/moby/moby/v2/testutil/daemon"
	"github.com/moby/moby/v2/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestPingCacheHeaders(t *testing.T) {
	ctx := setupTest(t)

	res, _, err := request.Get(ctx, "/_ping")
	assert.NilError(t, err)
	assert.Equal(t, res.StatusCode, http.StatusOK)

	assert.Assert(t, is.DeepEqual(res.Header.Values("Cache-Control"), []string{"no-cache, no-store, must-revalidate"}))
	assert.Assert(t, is.DeepEqual(res.Header.Values("Pragma"), []string{"no-cache"}))
}

func TestPingGet(t *testing.T) {
	ctx := setupTest(t)

	res, body, err := request.Get(ctx, "/_ping")
	assert.NilError(t, err)

	b, err := request.ReadBody(body)
	assert.NilError(t, err)
	assert.Equal(t, string(b), "OK")
	assert.Equal(t, res.StatusCode, http.StatusOK)
	assert.Check(t, res.Header.Get("Api-Version") != "")
}

func TestPingHead(t *testing.T) {
	ctx := setupTest(t)

	res, body, err := request.Head(ctx, "/_ping")
	assert.NilError(t, err)

	b, err := request.ReadBody(body)
	assert.NilError(t, err)
	assert.Equal(t, 0, len(b))
	assert.Equal(t, res.StatusCode, http.StatusOK)
	assert.Check(t, res.Header.Get("Api-Version") != "")
}

func TestPingSwarmHeader(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartNode(t)
	defer d.Stop(t)
	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	t.Run("before swarm init", func(t *testing.T) {
		ctx := testutil.StartSpan(ctx, t)
		p, err := apiClient.Ping(ctx)
		assert.NilError(t, err)
		assert.Equal(t, p.SwarmStatus.NodeState, swarm.LocalNodeStateInactive)
		assert.Equal(t, p.SwarmStatus.ControlAvailable, false)
	})

	_, err := apiClient.SwarmInit(ctx, swarm.InitRequest{ListenAddr: "127.0.0.1", AdvertiseAddr: "127.0.0.1:2377"})
	assert.NilError(t, err)

	t.Run("after swarm init", func(t *testing.T) {
		ctx := testutil.StartSpan(ctx, t)
		p, err := apiClient.Ping(ctx)
		assert.NilError(t, err)
		assert.Equal(t, p.SwarmStatus.NodeState, swarm.LocalNodeStateActive)
		assert.Equal(t, p.SwarmStatus.ControlAvailable, true)
	})

	err = apiClient.SwarmLeave(ctx, true)
	assert.NilError(t, err)

	t.Run("after swarm leave", func(t *testing.T) {
		ctx := testutil.StartSpan(ctx, t)
		p, err := apiClient.Ping(ctx)
		assert.NilError(t, err)
		assert.Equal(t, p.SwarmStatus.NodeState, swarm.LocalNodeStateInactive)
		assert.Equal(t, p.SwarmStatus.ControlAvailable, false)
	})
}

func TestPingBuilderHeader(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "cannot spin up additional daemons on windows")

	ctx := setupTest(t)
	d := daemon.New(t)
	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	t.Run("default config", func(t *testing.T) {
		testutil.StartSpan(ctx, t)
		d.Start(t)
		defer d.Stop(t)

		expected := build.BuilderBuildKit
		if runtime.GOOS == "windows" {
			expected = build.BuilderV1
		}

		p, err := apiClient.Ping(ctx)
		assert.NilError(t, err)
		assert.Equal(t, p.BuilderVersion, expected)
	})

	t.Run("buildkit disabled", func(t *testing.T) {
		testutil.StartSpan(ctx, t)
		cfg := filepath.Join(d.RootDir(), "daemon.json")
		err := os.WriteFile(cfg, []byte(`{"features": { "buildkit": false }}`), 0o644)
		assert.NilError(t, err)
		d.Start(t, "--config-file", cfg)
		defer d.Stop(t)

		expected := build.BuilderV1
		p, err := apiClient.Ping(ctx)
		assert.NilError(t, err)
		assert.Equal(t, p.BuilderVersion, expected)
	})
}
