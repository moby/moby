package v2

import (
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/restartmanager"
)

// Plugin represents an individual plugin.
type Plugin struct {
	sync.RWMutex
	PluginObj         types.Plugin                  `json:"plugin"`
	PClient           *plugins.Client               `json:"-"`
	RestartManager    restartmanager.RestartManager `json:"-"`
	RuntimeSourcePath string                        `json:"-"`
	ExitChan          chan bool                     `json:"-"`
	RefCount          int                           `json:"-"`
}
