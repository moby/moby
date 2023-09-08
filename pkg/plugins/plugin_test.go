package plugins // import "github.com/docker/docker/pkg/plugins"

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/pkg/plugins/transport"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

const (
	fruitPlugin     = "fruit"
	fruitImplements = "apple"
)

// regression test for deadlock in handlers
func TestPluginAddHandler(t *testing.T) {
	t.Parallel()
	// make a plugin which is pre-activated
	p := &Plugin{activateWait: sync.NewCond(&sync.Mutex{})}
	p.Manifest = &Manifest{Implements: []string{"bananas"}}
	storage.Lock()
	storage.plugins["qwerty"] = p
	storage.Unlock()

	testActive(t, p)
	Handle("bananas", func(_ string, _ *Client) {})
	testActive(t, p)
}

func TestPluginWaitBadPlugin(t *testing.T) {
	p := &Plugin{activateWait: sync.NewCond(&sync.Mutex{})}
	p.activateErr = errors.New("some junk happened")
	testActive(t, p)
}

func testActive(t *testing.T, p *Plugin) {
	done := make(chan struct{})
	go func() {
		p.waitActive()
		close(done)
	}()

	select {
	case <-time.After(100 * time.Millisecond):
		_, f, l, _ := runtime.Caller(1)
		t.Fatalf("%s:%d: deadlock in waitActive", filepath.Base(f), l)
	case <-done:
	}
}

func TestGet(t *testing.T) {
	// TODO: t.Parallel()
	// TestPluginWithNoManifest also registers fruitPlugin

	p := &Plugin{name: fruitPlugin, activateWait: sync.NewCond(&sync.Mutex{})}
	p.Manifest = &Manifest{Implements: []string{fruitImplements}}
	storage.Lock()
	storage.plugins[fruitPlugin] = p
	storage.Unlock()

	t.Run("success", func(t *testing.T) {
		plugin, err := Get(fruitPlugin, fruitImplements)
		assert.NilError(t, err)

		assert.Check(t, is.Equal(p.Name(), plugin.Name()))
		assert.Check(t, is.Nil(plugin.Client()))
		assert.Check(t, plugin.IsV1())
	})

	// check negative case where plugin fruit doesn't implement banana
	t.Run("not implemented", func(t *testing.T) {
		_, err := Get("fruit", "banana")
		assert.Check(t, is.ErrorIs(err, ErrNotImplements))
	})

	// check negative case where plugin vegetable doesn't exist
	t.Run("not exists", func(t *testing.T) {
		_, err := Get(testNonExistingPlugin, "no-such-implementation")
		assert.Check(t, is.ErrorIs(err, ErrNotFound))
	})
}

func TestPluginWithNoManifest(t *testing.T) {
	// TODO: t.Parallel()
	// TestGet also registers fruitPlugin
	mux, addr := setupRemotePluginServer(t)

	m := Manifest{[]string{fruitImplements}}
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(m)
	assert.NilError(t, err)

	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		assert.Assert(t, is.Equal(r.Method, http.MethodPost))

		header := w.Header()
		header.Set("Content-Type", transport.VersionMimetype)

		io.Copy(w, &buf)
	})

	p := &Plugin{
		name:         fruitPlugin,
		activateWait: sync.NewCond(&sync.Mutex{}),
		Addr:         addr,
		TLSConfig:    &tlsconfig.Options{InsecureSkipVerify: true},
	}
	storage.Lock()
	storage.plugins[fruitPlugin] = p
	storage.Unlock()

	plugin, err := Get(fruitPlugin, fruitImplements)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(p.name, plugin.Name()))
}

func TestGetAll(t *testing.T) {
	t.Parallel()

	tmpdir := t.TempDir()
	r := LocalRegistry{
		socketsPath: tmpdir,
		specsPaths:  []string{tmpdir},
	}

	p := filepath.Join(tmpdir, "example.json")
	spec := `{
	"Name": "example",
	"Addr": "https://example.com/docker/plugin"
}`

	err := os.WriteFile(p, []byte(spec), 0o644)
	assert.NilError(t, err)

	plugin, err := r.Plugin("example")
	assert.NilError(t, err)

	plugin.Manifest = &Manifest{Implements: []string{"apple"}}
	storage.Lock()
	storage.plugins["example"] = plugin
	storage.Unlock()

	fetchedPlugins, err := r.GetAll("apple")
	assert.NilError(t, err)
	assert.Check(t, is.Len(fetchedPlugins, 1))
	assert.Check(t, is.Equal(fetchedPlugins[0].Name(), plugin.Name()))
}
