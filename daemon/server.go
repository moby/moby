package daemon

import (
	"github.com/dotcloud/docker/utils"
)

type Server interface {
	LogEvent(action, id, from string) *utils.JSONMessage
	IsRunning() bool // returns true if the server is currently in operation
}
