package v2

import (
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/plugins"
)

// Plugin represents an individual plugin.
type Plugin struct {
	sync.RWMutex
	PluginObj         types.Plugin    `json:"plugin"`
	PClient           *plugins.Client `json:"-"`
	RuntimeSourcePath string          `json:"-"`
	RefCount          int             `json:"-"`
	Restart           bool            `json:"-"`
	ExitChan          chan bool       `json:"-"`
}
