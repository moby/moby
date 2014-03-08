package runtime

import (
	"github.com/dotcloud/docker/utils"
)

type Server interface {
	LogEvent(action, id, from string) *utils.JSONMessage
}
