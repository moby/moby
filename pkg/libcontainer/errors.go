package libcontainer

import (
	"errors"
)

var (
	ErrInvalidPid = errors.New("no ns pid found")
)
