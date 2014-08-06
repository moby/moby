package daemon

import (
	"github.com/docker/docker/utils"
)

type Server interface {
	LogEvent(action, id, from string) *utils.JSONMessage
}
