package system // import "github.com/docker/docker/integration/system"

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestPingCacheHeaders(t *testing.T) {
	ctx := setupTest(t)

	res, _, err := request.Get(ctx, "/_ping")
	assert.NilError(t, err)
	assert.Equal(t, res.StatusCode, http.StatusOK)

	assert.Equal(t, hdr(res, "Cache-Control"), "no-cache, no-store, must-revalidate")
	assert.Equal(t, hdr(res, "Pragma"), "no-cache")
}

func TestPingGet(t *testing.T) {
	ctx := setupTest(t)

	res, body, err := request.Get(ctx, "/_ping")
	assert.NilError(t, err)

	b, err := request.ReadBody(body)
	assert.NilError(t, err)
	assert.Equal(t, string(b), "OK")
	assert.Equal(t, res.StatusCode, http.StatusOK)
	assert.Check(t, hdr(res, "API-Version") != "")
}

func TestPingHead(t *testing.T) {
	ctx := setupTest(t)

	res, body, err := request.Head(ctx, "/_ping")
	assert.NilError(t, err)

	b, err := request.ReadBody(body)
	assert.NilError(t, err)
	assert.Equal(t, 0, len(b))
	assert.Equal(t, res.StatusCode, http.StatusOK)
	assert.Check(t, hdr(res, "API-Version") != "")
}

func TestPingSwarmHeader(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)
	d := daemon.New(t)
	d.Start(t)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	t.Run("before swarm init", func(t *testing.T) {
		ctx := testutil.StartSpan(ctx, t)
		p, err := client.Ping(ctx)
		assert.NilError(t, err)
		assert.Equal(t, p.SwarmStatus.NodeState, swarm.LocalNodeStateInactive)
		assert.Equal(t, p.SwarmStatus.ControlAvailable, false)
	})

	_, err := client.SwarmInit(ctx, swarm.InitRequest{ListenAddr: "127.0.0.1", AdvertiseAddr: "127.0.0.1:2377"})
	assert.NilError(t, err)

	t.Run("after swarm init", func(t *testing.T) {
		ctx := testutil.StartSpan(ctx, t)
		p, err := client.Ping(ctx)
		assert.NilError(t, err)
		assert.Equal(t, p.SwarmStatus.NodeState, swarm.LocalNodeStateActive)
		assert.Equal(t, p.SwarmStatus.ControlAvailable, true)
	})

	err = client.SwarmLeave(ctx, true)
	assert.NilError(t, err)

	t.Run("after swarm leave", func(t *testing.T) {
		ctx := testutil.StartSpan(ctx, t)
		p, err := client.Ping(ctx)
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
	client := d.NewClientT(t)
	defer client.Close()

	t.Run("default config", func(t *testing.T) {
		testutil.StartSpan(ctx, t)
		d.Start(t)
		defer d.Stop(t)

		expected := types.BuilderBuildKit
		if runtime.GOOS == "windows" {
			expected = types.BuilderV1
		}

		p, err := client.Ping(ctx)
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

		expected := types.BuilderV1
		p, err := client.Ping(ctx)
		assert.NilError(t, err)
		assert.Equal(t, p.BuilderVersion, expected)
	})
}

func hdr(res *http.Response, name string) string {
	val, ok := res.Header[http.CanonicalHeaderKey(name)]
	if !ok || len(val) == 0 {
		return ""
	}
	return strings.Join(val, ", ")
}
