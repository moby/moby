//go:build !dfrundevice

package dockerfile2llb

import (
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
)

func dispatchRunDevices(_ *instructions.RunCommand) ([]llb.RunOption, error) {
	return nil, nil
}
