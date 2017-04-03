package plugin // import "github.com/docker/docker/integration/plugin"

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/internal/test/daemon"
	"github.com/docker/docker/internal/test/fixtures/plugin"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
)

func TestPluginSaveLoad(t *testing.T) {
	t.Parallel()

	d := daemon.New(t)
	defer d.Cleanup(t)
	d.Start(t, "--iptables=false")
	defer d.Stop(t)
	client := d.NewClientT(t)

	ctx := context.Background()
	assert.Assert(t, plugin.Create(ctx, client, "test"))

	f, err := ioutil.TempFile("", t.Name())
	assert.Assert(t, err)
	defer os.Remove(f.Name())

	rdr, err := client.PluginSave(ctx, "test")
	assert.Assert(t, err)
	defer rdr.Close()

	copyTimeout(t, f, rdr, 60*time.Second)

	err = client.PluginRemove(ctx, "test", types.PluginRemoveOptions{Force: true})
	assert.Assert(t, err)

	_, err = f.Seek(0, io.SeekStart)
	assert.Assert(t, err)

	pr, err := client.PluginLoad(ctx, f)
	assert.Assert(t, err)
	defer pr.Body.Close()

	copyTimeout(t, ioutil.Discard, pr.Body, 60*time.Second)

	p, _, err := client.PluginInspectWithRaw(ctx, "test")
	assert.Assert(t, err)
	assert.Assert(t, is.Equal(p.Name, "test"))
}

func copyTimeout(t *testing.T, dst io.Writer, src io.Reader, dur time.Duration) {
	t.Helper()
	chErr := make(chan error)
	go func() {
		_, err := io.Copy(dst, src)
		chErr <- err
	}()
	select {
	case <-time.After(dur):
		t.Fatal()
	case err := <-chErr:
		assert.Assert(t, err)
	}
}
