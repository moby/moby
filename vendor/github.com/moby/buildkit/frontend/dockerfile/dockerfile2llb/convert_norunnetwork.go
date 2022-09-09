//go:build !dfrunnetwork
// +build !dfrunnetwork

package dockerfile2llb

import (
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
)

func dispatchRunNetwork(c *instructions.RunCommand) (llb.RunOption, error) {
	return nil, nil
}
