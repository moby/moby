package daemon

import (
	"errors"

	"github.com/moby/moby/v2/daemon/config"
)

type runtimes struct{}

func (r *runtimes) Get(name string) (string, any, error) {
	return "", nil, errors.New("not implemented")
}

func initRuntimesDir(*config.Config) error {
	return nil
}

func setupRuntimes(*config.Config) (runtimes, error) {
	return runtimes{}, nil
}
