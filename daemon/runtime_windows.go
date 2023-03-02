package daemon

import (
	"errors"
)

func (daemon *Daemon) getRuntime(name string) (shim string, opts interface{}, err error) {
	return "", nil, errors.New("not implemented")
}
