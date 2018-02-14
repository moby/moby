package image

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/docker/docker/integration-cli/registry"
	"github.com/stretchr/testify/assert"
)

// Test --registry-mirrors with unix:// sockets.
// Requires Internet connection.
func TestImageRegistryMirrorsUNIX(t *testing.T) {
	t.Parallel()
	tmp, err := ioutil.TempDir("", "test-image-registry-mirrors")
	assert.NoError(t, err)
	defer os.RemoveAll(tmp)
	registrySocket := filepath.Join(tmp, "registry.sock")
	registryURL := "unix://" + registrySocket
	testImageRegistryMirrors(t, registryURL)
}

// Test --registry-mirrors with http:// sockets.
// Requires Internet connection.
func TestImageRegistryMirrorsHTTP(t *testing.T) {
	// FIXME(AkihiroSuda): this is also defined in the legacy integration-cli (`privateRegistryURL`)
	registryURL := "http://127.0.0.1:5000"
	testImageRegistryMirrors(t, registryURL)
}

func testImageRegistryMirrors(t *testing.T, registryURL string) {
	registry, err := registry.NewV2(registry.V2Config{
		RegistryURL:    registryURL,
		ProxyRemoteURL: "https://registry-1.docker.io",
	})
	assert.NoError(t, err)
	defer registry.Close()

	// Wait for registry to be ready to serve requests.
	for i := 0; i != 50; i++ {
		if err = registry.Ping(); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.NoError(t, err)

	d := daemon.New(t, "", "dockerd", daemon.Config{})
	d.Start(t, "--registry-mirror="+registryURL)
	defer d.Stop(t)

	client, err := d.NewClient()
	assert.NoError(t, err, "error creating client")

	ctx := context.Background()
	rc, err := client.ImagePull(ctx, "hello-world", types.ImagePullOptions{})
	assert.NoError(t, err)
	_, err = io.Copy(ioutil.Discard, rc)
	assert.NoError(t, err)
	rc.Close()

	// Make sure the image is cached to the local registry
	manifestDigest, err := registry.ManifestDigest("library/hello-world", "latest")
	assert.NoError(t, err)
	assert.NotEmpty(t, manifestDigest)
}
