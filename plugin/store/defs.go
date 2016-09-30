package store

import (
	"path/filepath"
	"sync"

	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/plugin/v2"
)

// Store manages the plugin inventory in memory and on-disk
type Store struct {
	sync.RWMutex
	plugins map[string]*v2.Plugin
	/* handlers are necessary for transition path of legacy plugins
	 * to the new model. Legacy plugins use Handle() for registering an
	 * activation callback.*/
	handlers map[string]func(string, *plugins.Client)
	nameToID map[string]string
	plugindb string
}

// NewStore creates a Store.
func NewStore(libRoot string) *Store {
	return &Store{
		plugins:  make(map[string]*v2.Plugin),
		handlers: make(map[string]func(string, *plugins.Client)),
		nameToID: make(map[string]string),
		plugindb: filepath.Join(libRoot, "plugins", "plugins.json"),
	}
}
