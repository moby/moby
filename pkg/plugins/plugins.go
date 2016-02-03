// Package plugins provides structures and helper functions to manage Docker
// plugins.
//
// Docker discovers plugins by looking for them in the plugin directory whenever
// a user or container tries to use one by name. UNIX domain socket files must
// be located under /run/docker/plugins, whereas spec files can be located
// either under /etc/docker/plugins or /usr/lib/docker/plugins. This is handled
// by the Registry interface, which lets you list all plugins or get a plugin by
// its name if it exists.
//
// The plugins need to implement an HTTP server and bind this to the UNIX socket
// or the address specified in the spec files.
// A handshake is send at /Plugin.Activate, and plugins are expected to return
// a Manifest with a list of of Docker subsystems which this plugin implements.
//
// In order to use a plugins, you can use the ``Get`` with the name of the
// plugin and the subsystem it implements.
//
//	plugin, err := plugins.Get("example", "VolumeDriver")
//	if err != nil {
//		return fmt.Errorf("Error looking up volume plugin example: %v", err)
//	}
package plugins

import (
	"errors"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/go-connections/tlsconfig"
)

var (
	// ErrNotImplements is returned if the plugin does not implement the requested driver.
	ErrNotImplements = errors.New("Plugin does not implement the requested driver")
)

type plugins struct {
	sync.Mutex
	plugins map[string]*Plugin
}

var (
	storage          = plugins{plugins: make(map[string]*Plugin)}
	extpointHandlers = make(map[string]func(string, *Client))
)

// Manifest lists what a plugin implements.
type Manifest struct {
	// List of subsystem the plugin implements.
	Implements []string
}

// Plugin is the definition of a docker plugin.
type Plugin struct {
	// Name of the plugin
	Name string `json:"-"`
	// Address of the plugin
	Addr string
	// TLS configuration of the plugin
	TLSConfig tlsconfig.Options
	// Client attached to the plugin
	Client *Client `json:"-"`
	// Manifest of the plugin (see above)
	Manifest *Manifest `json:"-"`

	activatErr   error
	activateOnce sync.Once
}

func newLocalPlugin(name, addr string) *Plugin {
	return &Plugin{
		Name:      name,
		Addr:      addr,
		TLSConfig: tlsconfig.Options{InsecureSkipVerify: true},
	}
}

func (p *Plugin) activate() error {
	p.activateOnce.Do(func() {
		p.activatErr = p.activateWithLock()
	})
	return p.activatErr
}

func (p *Plugin) activateWithLock() error {
	c, err := NewClient(p.Addr, p.TLSConfig)
	if err != nil {
		return err
	}
	p.Client = c

	m := new(Manifest)
	if err = p.Client.Call("Plugin.Activate", nil, m); err != nil {
		return err
	}

	p.Manifest = m

	for _, iface := range m.Implements {
		handler, handled := extpointHandlers[iface]
		if !handled {
			continue
		}
		handler(p.Name, p.Client)
	}
	return nil
}

func (p *Plugin) implements(kind string) bool {
	for _, driver := range p.Manifest.Implements {
		if driver == kind {
			return true
		}
	}
	return false
}

func load(name string) (*Plugin, error) {
	return loadWithRetry(name, true)
}

func loadWithRetry(name string, retry bool) (*Plugin, error) {
	registry := newLocalRegistry()
	start := time.Now()

	var retries int
	for {
		pl, err := registry.Plugin(name)
		if err != nil {
			if !retry || err == ErrNotFound {
				return nil, err
			}

			timeOff := backoff(retries)
			if abort(start, timeOff) {
				return nil, err
			}
			retries++
			logrus.Warnf("Unable to locate plugin: %s, retrying in %v", name, timeOff)
			time.Sleep(timeOff)
			continue
		}

		storage.Lock()
		storage.plugins[name] = pl
		storage.Unlock()

		err = pl.activate()

		if err != nil {
			storage.Lock()
			delete(storage.plugins, name)
			storage.Unlock()
		}

		return pl, err
	}
}

func get(name string) (*Plugin, error) {
	storage.Lock()
	pl, ok := storage.plugins[name]
	storage.Unlock()
	if ok {
		return pl, pl.activate()
	}
	return load(name)
}

// Get returns the plugin given the specified name and requested implementation.
func Get(name, imp string) (*Plugin, error) {
	pl, err := get(name)
	if err != nil {
		return nil, err
	}
	if pl.implements(imp) {
		logrus.Debugf("%s implements: %s", name, imp)
		return pl, nil
	}
	return nil, ErrNotImplements
}

// Handle adds the specified function to the extpointHandlers.
func Handle(iface string, fn func(string, *Client)) {
	extpointHandlers[iface] = fn
}

// GetAll returns all the plugins for the specified implementation
func GetAll(imp string) ([]*Plugin, error) {
	pluginNames, err := Scan()
	if err != nil {
		return nil, err
	}

	type plLoad struct {
		pl  *Plugin
		err error
	}

	chPl := make(chan plLoad, len(pluginNames))
	for _, name := range pluginNames {
		go func(name string) {
			pl, err := loadWithRetry(name, false)
			chPl <- plLoad{pl, err}
		}(name)
	}

	var out []*Plugin
	for i := 0; i < len(pluginNames); i++ {
		pl := <-chPl
		if pl.err != nil {
			logrus.Error(err)
			continue
		}
		if pl.pl.implements(imp) {
			out = append(out, pl.pl)
		}
	}
	return out, nil
}
