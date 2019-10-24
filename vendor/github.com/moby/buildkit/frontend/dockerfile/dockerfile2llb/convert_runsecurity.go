// +build dfrunsecurity

package dockerfile2llb

import (
	"github.com/pkg/errors"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/solver/pb"
)

func dispatchRunSecurity(d *dispatchState, c *instructions.RunCommand) error {
	security := instructions.GetSecurity(c)

	for _, sec := range security {
		switch sec {
		case instructions.SecurityInsecure:
			d.state = d.state.Security(pb.SecurityMode_INSECURE)
		case instructions.SecuritySandbox:
			d.state = d.state.Security(pb.SecurityMode_SANDBOX)
		default:
			return errors.Errorf("unsupported security mode %q", sec)
		}
	}

	return nil
}
