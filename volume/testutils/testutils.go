package testutils // import "github.com/docker/docker/volume/testutils"

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/volume"
)

// NoopVolume is a volume that doesn't perform any operation
type NoopVolume struct{}

// Name is the name of the volume
func (NoopVolume) Name() string { return "noop" }

// DriverName is the name of the driver
func (NoopVolume) DriverName() string { return "noop" }

// Path is the filesystem path to the volume
func (NoopVolume) Path() string { return "noop" }

// Mount mounts the volume in the container
func (NoopVolume) Mount(_ string) (string, error) { return "noop", nil }

// Unmount unmounts the volume from the container
func (NoopVolume) Unmount(_ string) error { return nil }

// Status provides low-level details about the volume
func (NoopVolume) Status() map[string]interface{} { return nil }

// CreatedAt provides the time the volume (directory) was created at
func (NoopVolume) CreatedAt() (time.Time, error) { return time.Now(), nil }

// FakeVolume is a fake volume with a random name
type FakeVolume struct {
	name       string
	driverName string
	createdAt  time.Time
}

// NewFakeVolume creates a new fake volume for testing
func NewFakeVolume(name string, driverName string) volume.Volume {
	return FakeVolume{name: name, driverName: driverName, createdAt: time.Now()}
}

// Name is the name of the volume
func (f FakeVolume) Name() string { return f.name }

// DriverName is the name of the driver
func (f FakeVolume) DriverName() string { return f.driverName }

// Path is the filesystem path to the volume
func (FakeVolume) Path() string { return "fake" }

// Mount mounts the volume in the container
func (FakeVolume) Mount(_ string) (string, error) { return "fake", nil }

// Unmount unmounts the volume from the container
func (FakeVolume) Unmount(_ string) error { return nil }

// Status provides low-level details about the volume
func (FakeVolume) Status() map[string]interface{} {
	return map[string]interface{}{"datakey": "datavalue"}
}

// CreatedAt provides the time the volume (directory) was created at
func (f FakeVolume) CreatedAt() (time.Time, error) {
	return f.createdAt, nil
}

// FakeDriver is a driver that generates fake volumes
type FakeDriver struct {
	name string
	vols map[string]volume.Volume
}

// NewFakeDriver creates a new FakeDriver with the specified name
func NewFakeDriver(name string) volume.Driver {
	return &FakeDriver{
		name: name,
		vols: make(map[string]volume.Volume),
	}
}

// Name is the name of the driver
func (d *FakeDriver) Name() string { return d.name }

// Create initializes a fake volume.
// It returns an error if the options include an "error" key with a message
func (d *FakeDriver) Create(name string, opts map[string]string) (volume.Volume, error) {
	if opts != nil && opts["error"] != "" {
		return nil, fmt.Errorf(opts["error"])
	}
	v := NewFakeVolume(name, d.name)
	d.vols[name] = v
	return v, nil
}

// Remove deletes a volume.
func (d *FakeDriver) Remove(v volume.Volume) error {
	if _, exists := d.vols[v.Name()]; !exists {
		return fmt.Errorf("no such volume")
	}
	delete(d.vols, v.Name())
	return nil
}

// List lists the volumes
func (d *FakeDriver) List() ([]volume.Volume, error) {
	var vols []volume.Volume
	for _, v := range d.vols {
		vols = append(vols, v)
	}
	return vols, nil
}

// Get gets the volume
func (d *FakeDriver) Get(name string) (volume.Volume, error) {
	if v, exists := d.vols[name]; exists {
		return v, nil
	}
	return nil, fmt.Errorf("no such volume")
}

// Scope returns the local scope
func (*FakeDriver) Scope() string {
	return "local"
}

type fakePlugin struct {
	client *plugins.Client
	name   string
	refs   int
}

// MakeFakePlugin creates a fake plugin from the passed in driver
// Note: currently only "Create" is implemented because that's all that's needed
// so far. If you need it to test something else, add it here, but probably you
// shouldn't need to use this except for very specific cases with v2 plugin handling.
func MakeFakePlugin(d volume.Driver, l net.Listener) (plugingetter.CompatPlugin, error) {
	c, err := plugins.NewClient(l.Addr().Network()+"://"+l.Addr().String(), nil)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()

	mux.HandleFunc("/VolumeDriver.Create", func(w http.ResponseWriter, r *http.Request) {
		createReq := struct {
			Name string
			Opts map[string]string
		}{}
		if err := json.NewDecoder(r.Body).Decode(&createReq); err != nil {
			fmt.Fprintf(w, `{"Err": "%s"}`, err.Error())
			return
		}
		_, err := d.Create(createReq.Name, createReq.Opts)
		if err != nil {
			fmt.Fprintf(w, `{"Err": "%s"}`, err.Error())
			return
		}
		w.Write([]byte("{}"))
	})

	go http.Serve(l, mux) // #nosec G114 -- Ignoring for test-code: G114: Use of net/http serve function that has no support for setting timeouts (gosec)
	return &fakePlugin{client: c, name: d.Name()}, nil
}

func (p *fakePlugin) Client() *plugins.Client {
	return p.client
}

func (p *fakePlugin) Name() string {
	return p.name
}

func (p *fakePlugin) IsV1() bool {
	return false
}

func (p *fakePlugin) ScopedPath(s string) string {
	return s
}

type fakePluginGetter struct {
	plugins map[string]plugingetter.CompatPlugin
}

// NewFakePluginGetter returns a plugin getter for fake plugins
func NewFakePluginGetter(pls ...plugingetter.CompatPlugin) plugingetter.PluginGetter {
	idx := make(map[string]plugingetter.CompatPlugin, len(pls))
	for _, p := range pls {
		idx[p.Name()] = p
	}
	return &fakePluginGetter{plugins: idx}
}

// This ignores the second argument since we only care about volume drivers here,
// there shouldn't be any other kind of plugin in here
func (g *fakePluginGetter) Get(name, _ string, mode int) (plugingetter.CompatPlugin, error) {
	p, ok := g.plugins[name]
	if !ok {
		return nil, errors.New("not found")
	}
	p.(*fakePlugin).refs += mode
	return p, nil
}

func (g *fakePluginGetter) GetAllByCap(capability string) ([]plugingetter.CompatPlugin, error) {
	panic("GetAllByCap shouldn't be called")
}

func (g *fakePluginGetter) GetAllManagedPluginsByCap(capability string) []plugingetter.CompatPlugin {
	panic("GetAllManagedPluginsByCap should not be called")
}

func (g *fakePluginGetter) Handle(capability string, callback func(string, *plugins.Client)) {
	panic("Handle should not be called")
}

// FakeRefs checks ref count on a fake plugin.
func FakeRefs(p plugingetter.CompatPlugin) int {
	// this should panic if something other than a `*fakePlugin` is passed in
	return p.(*fakePlugin).refs
}
