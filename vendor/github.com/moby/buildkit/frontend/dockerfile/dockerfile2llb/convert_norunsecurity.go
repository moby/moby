// +build !dfrunsecurity

package dockerfile2llb

import (
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
)

func dispatchRunSecurity(d *dispatchState, c *instructions.RunCommand) error {
	return nil
}
