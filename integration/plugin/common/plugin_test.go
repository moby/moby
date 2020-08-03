package common // import "github.com/docker/docker/integration/plugin/common"

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"path"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/docker/testutil/fixtures/plugin"
	"github.com/docker/docker/testutil/registry"
	"github.com/docker/docker/testutil/request"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestPluginInvalidJSON(t *testing.T) {
	defer setupTest(t)()

	endpoints := []string{"/plugins/foobar/set"}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			t.Parallel()

			res, body, err := request.Post(ep, request.RawString("{invalid json"), request.JSON)
			assert.NilError(t, err)
			assert.Equal(t, res.StatusCode, http.StatusBadRequest)

			buf, err := request.ReadBody(body)
			assert.NilError(t, err)
			assert.Check(t, is.Contains(string(buf), "invalid character 'i' looking for beginning of object key string"))

			res, body, err = request.Post(ep, request.JSON)
			assert.NilError(t, err)
			assert.Equal(t, res.StatusCode, http.StatusBadRequest)

			buf, err = request.ReadBody(body)
			assert.NilError(t, err)
			assert.Check(t, is.Contains(string(buf), "got EOF while reading request body"))
		})
	}
}

func TestPluginInstall(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.OSType == "windows")
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

		_, err = io.Copy(ioutil.Discard, rdr)
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
		auth := &types.AuthConfig{ServerAddress: registry.DefaultURL, Username: "testuser", Password: "testpassword"}
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

		_, err = io.Copy(ioutil.Discard, rdr)
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

		_, err = io.Copy(ioutil.Discard, rdr)
		assert.NilError(t, err)

		_, _, err = client.PluginInspectWithRaw(ctx, repo)
		assert.NilError(t, err)
	})
	// TODO: test insecure registry with https
}
