package plugins

import (
	"errors"
	"sync"

	"github.com/Sirupsen/logrus"
)

var (
	ErrNotImplements  = errors.New("Plugin does not implement the requested driver")
	ErrNotInitialized = errors.New("Plugins registry not initialized")

	DefaultRegistry Registry
)

type plugins struct {
	sync.Mutex
	plugins map[string]*Plugin
}

var (
	extpointHandlers = make(map[string]func(string, *Client))
)

type Manifest struct {
	Implements []string
}

type Plugin struct {
	Name     string
	Addr     string
	Client   *Client
	Manifest *Manifest
}

func (p *Plugin) activate() error {
	m := new(Manifest)
	p.Client = NewClient(p.Addr)
	err := p.Client.Call("Plugin.Activate", nil, m)
	if err != nil {
		return err
	}

	logrus.Debugf("%s's manifest: %v", p.Name, m)
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

func Get(name, impl string) (*Plugin, error) {
	if DefaultRegistry == nil {
		return nil, ErrNotInitialized
	}
	return DefaultRegistry.Get(name, impl)
}

func Handle(iface string, fn func(string, *Client)) {
	extpointHandlers[iface] = fn
}
