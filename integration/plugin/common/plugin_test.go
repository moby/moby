package common // import "github.com/docker/docker/integration/plugin/common"

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/docker/testutil/fixtures/plugin"
	"github.com/docker/docker/testutil/registry"
	"github.com/docker/docker/testutil/request"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// TestPluginInvalidJSON tests that POST endpoints that expect a body return
// the correct error when sending invalid JSON requests.
func TestPluginInvalidJSON(t *testing.T) {
	t.Cleanup(setupTest(t))

	// POST endpoints that accept / expect a JSON body;
	endpoints := []string{
		"/plugins/foobar/set",
		"/plugins/foobar/upgrade",
		"/plugins/pull",
	}

	for _, ep := range endpoints {
		ep := ep
		t.Run(ep[1:], func(t *testing.T) {
			t.Parallel()

			t.Run("invalid content type", func(t *testing.T) {
				res, body, err := request.Post(ep, request.RawString("[]"), request.ContentType("text/plain"))
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "unsupported Content-Type header (text/plain): must be 'application/json'"))
			})

			t.Run("invalid JSON", func(t *testing.T) {
				res, body, err := request.Post(ep, request.RawString("{invalid json"), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "invalid JSON: invalid character 'i' looking for beginning of object key string"))
			})

			t.Run("extra content after JSON", func(t *testing.T) {
				res, body, err := request.Post(ep, request.RawString(`[] trailing content`), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(res.StatusCode, http.StatusBadRequest))

				buf, err := request.ReadBody(body)
				assert.NilError(t, err)
				assert.Check(t, is.Contains(string(buf), "unexpected content after JSON"))
			})

			t.Run("empty body", func(t *testing.T) {
				// empty body should not produce an 500 internal server error, or
				// any 5XX error (this is assuming the request does not produce
				// an internal server error for another reason, but it shouldn't)
				res, _, err := request.Post(ep, request.RawString(``), request.JSON)
				assert.NilError(t, err)
				assert.Check(t, res.StatusCode < http.StatusInternalServerError)
			})
		})
	}
}

func TestPluginInstall(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of localhost")

	ctx := context.Background()
	client := testEnv.APIClient()

	t.Run("no auth", func(t *testing.T) {
		defer setupTest(t)()

		reg := registry.NewV2(t)
		defer reg.Close()

		name := "test-" + strings.ToLower(t.Name())
		repo := path.Join(registry.DefaultURL, name+":latest")
		assert.NilError(t, plugin.CreateInRegistry(ctx, repo, nil))

		rdr, err := client.PluginInstall(ctx, repo, types.PluginInstallOptions{Disabled: true, RemoteRef: repo})
		assert.NilError(t, err)
		defer rdr.Close()

		_, err = io.Copy(io.Discard, rdr)
		assert.NilError(t, err)

		_, _, err = client.PluginInspectWithRaw(ctx, repo)
		assert.NilError(t, err)
	})

	t.Run("with htpasswd", func(t *testing.T) {
		defer setupTest(t)()

		reg := registry.NewV2(t, registry.Htpasswd)
		defer reg.Close()

		name := "test-" + strings.ToLower(t.Name())
		repo := path.Join(registry.DefaultURL, name+":latest")
		auth := &registrytypes.AuthConfig{ServerAddress: registry.DefaultURL, Username: "testuser", Password: "testpassword"}
		assert.NilError(t, plugin.CreateInRegistry(ctx, repo, auth))

		authEncoded, err := json.Marshal(auth)
		assert.NilError(t, err)

		rdr, err := client.PluginInstall(ctx, repo, types.PluginInstallOptions{
			RegistryAuth: base64.URLEncoding.EncodeToString(authEncoded),
			Disabled:     true,
			RemoteRef:    repo,
		})
		assert.NilError(t, err)
		defer rdr.Close()

		_, err = io.Copy(io.Discard, rdr)
		assert.NilError(t, err)

		_, _, err = client.PluginInspectWithRaw(ctx, repo)
		assert.NilError(t, err)
	})
	t.Run("with insecure", func(t *testing.T) {
		skip.If(t, !testEnv.IsLocalDaemon())

		addrs, err := net.InterfaceAddrs()
		assert.NilError(t, err)

		var bindTo string
		for _, addr := range addrs {
			ip, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ip.IP.IsLoopback() || ip.IP.To4() == nil {
				continue
			}
			bindTo = ip.IP.String()
		}

		if bindTo == "" {
			t.Skip("No suitable interface to bind registry to")
		}

		regURL := bindTo + ":5000"

		d := daemon.New(t)
		defer d.Stop(t)

		d.Start(t, "--insecure-registry="+regURL)
		defer d.Stop(t)

		reg := registry.NewV2(t, registry.URL(regURL))
		defer reg.Close()

		name := "test-" + strings.ToLower(t.Name())
		repo := path.Join(regURL, name+":latest")
		assert.NilError(t, plugin.CreateInRegistry(ctx, repo, nil, plugin.WithInsecureRegistry(regURL)))

		client := d.NewClientT(t)
		rdr, err := client.PluginInstall(ctx, repo, types.PluginInstallOptions{Disabled: true, RemoteRef: repo})
		assert.NilError(t, err)
		defer rdr.Close()

		_, err = io.Copy(io.Discard, rdr)
		assert.NilError(t, err)

		_, _, err = client.PluginInspectWithRaw(ctx, repo)
		assert.NilError(t, err)
	})
	// TODO: test insecure registry with https
}

func TestPluginsWithRuntimes(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.IsRootless, "Test not supported on rootless due to buggy daemon setup in rootless mode due to daemon restart")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	dir, err := os.MkdirTemp("", t.Name())
	assert.NilError(t, err)
	defer os.RemoveAll(dir)

	d := daemon.New(t)
	defer d.Cleanup(t)

	d.Start(t)
	defer d.Stop(t)

	ctx := context.Background()
	client := d.NewClientT(t)

	assert.NilError(t, plugin.Create(ctx, client, "test:latest"))
	defer client.PluginRemove(ctx, "test:latest", types.PluginRemoveOptions{Force: true})

	assert.NilError(t, client.PluginEnable(ctx, "test:latest", types.PluginEnableOptions{Timeout: 30}))

	p := filepath.Join(dir, "myrt")
	script := fmt.Sprintf(`#!/bin/sh
	file="%s/success"
	if [ "$1" = "someArg" ]; then
		shift
		file="${file}_someArg"
	fi

	touch $file
	exec runc $@
	`, dir)

	assert.NilError(t, os.WriteFile(p, []byte(script), 0o777))

	type config struct {
		Runtimes map[string]system.Runtime `json:"runtimes"`
	}

	cfg, err := json.Marshal(config{
		Runtimes: map[string]system.Runtime{
			"myrt":     {Path: p},
			"myrtArgs": {Path: p, Args: []string{"someArg"}},
		},
	})
	configPath := filepath.Join(dir, "config.json")
	os.WriteFile(configPath, cfg, 0o644)

	t.Run("No Args", func(t *testing.T) {
		d.Restart(t, "--default-runtime=myrt", "--config-file="+configPath)
		_, err = os.Stat(filepath.Join(dir, "success"))
		assert.NilError(t, err)
	})

	t.Run("With Args", func(t *testing.T) {
		d.Restart(t, "--default-runtime=myrtArgs", "--config-file="+configPath)
		_, err = os.Stat(filepath.Join(dir, "success_someArg"))
		assert.NilError(t, err)
	})
}

func TestPluginBackCompatMediaTypes(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRootless, "Rootless has a different view of localhost (needed for test registry access)")

	defer setupTest(t)()

	reg := registry.NewV2(t)
	defer reg.Close()
	reg.WaitReady(t)

	repo := path.Join(registry.DefaultURL, strings.ToLower(t.Name())+":latest")

	client := testEnv.APIClient()

	ctx := context.Background()
	assert.NilError(t, plugin.Create(ctx, client, repo))

	rdr, err := client.PluginPush(ctx, repo, "")
	assert.NilError(t, err)
	defer rdr.Close()

	buf := &strings.Builder{}
	assert.NilError(t, jsonmessage.DisplayJSONMessagesStream(rdr, buf, 0, false, nil), buf)

	// Use custom header here because older versions of the registry do not
	// parse the accept header correctly and does not like the accept header
	// that the default resolver code uses. "Older registries" here would be
	// like the one currently included in the test suite.
	headers := http.Header{}
	headers.Add("Accept", images.MediaTypeDockerSchema2Manifest)

	resolver := docker.NewResolver(docker.ResolverOptions{
		Headers: headers,
	})
	assert.NilError(t, err)

	n, desc, err := resolver.Resolve(ctx, repo)
	assert.NilError(t, err, repo)

	fetcher, err := resolver.Fetcher(ctx, n)
	assert.NilError(t, err)

	rdr, err = fetcher.Fetch(ctx, desc)
	assert.NilError(t, err)
	defer rdr.Close()

	var m ocispec.Manifest
	assert.NilError(t, json.NewDecoder(rdr).Decode(&m))
	assert.Check(t, is.Equal(m.MediaType, images.MediaTypeDockerSchema2Manifest))
	assert.Check(t, is.Len(m.Layers, 1))
	assert.Check(t, is.Equal(m.Layers[0].MediaType, images.MediaTypeDockerSchema2LayerGzip))
}
