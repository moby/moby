package builder

import (
	"strings"

	"github.com/moby/buildkit/solver/errdefs"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/moby/buildkit/util/stack"
	"google.golang.org/grpc/codes"
)

var enabledCaps = map[string]struct{}{
	"moby.buildkit.frontend.inputs":      {},
	"moby.buildkit.frontend.subrequests": {},
}

func validateCaps(req string) (forward bool, err error) {
	if req == "" {
		return
	}
	caps := strings.Split(req, ",")
	for _, c := range caps {
		parts := strings.SplitN(c, "+", 2)
		if _, ok := enabledCaps[parts[0]]; !ok {
			err = stack.Enable(grpcerrors.WrapCode(errdefs.NewUnsupportedFrontendCapError(parts[0]), codes.Unimplemented))
			if strings.Contains(c, "+forward") {
				forward = true
			} else {
				return false, err
			}
		}
	}
	return
}
