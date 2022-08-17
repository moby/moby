package daemon

import (
	"errors"

	"github.com/docker/docker/daemon/config"
)

func (daemon *Daemon) getRuntime(cfg *config.Config, name string) (shim string, opts interface{}, err error) {
	return "", nil, errors.New("not implemented")
}
