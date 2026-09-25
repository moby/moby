//go:build no_embedded_containerd

package command

import (
	"context"
	"errors"
	"time"
)

func (cli *daemonCLI) initEmbeddedContainerd(context.Context) (func(time.Duration) error, error) {
	return nil, errors.New("embedded containerd is not supported in this build")
}
