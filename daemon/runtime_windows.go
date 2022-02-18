package daemon

import (
	"github.com/moby/moby/api/types"
	"github.com/pkg/errors"
)

func (daemon *Daemon) getRuntime(name string) (*types.Runtime, error) {
	return nil, errors.New("not implemented")
}
