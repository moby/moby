// +build dfrunmount,!dfsecrets

package dockerfile2llb

import (
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/pkg/errors"
)

func dispatchSecret(m *instructions.Mount) (llb.RunOption, error) {
	return nil, errors.Errorf("secret mounts not allowed")
}
