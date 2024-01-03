package errdefs

import (
	"context"
	"errors"
	"strings"

	"github.com/moby/buildkit/util/grpcerrors"
	"google.golang.org/grpc/codes"
)

func IsCanceled(ctx context.Context, err error) bool {
	if errors.Is(err, context.Canceled) || grpcerrors.Code(err) == codes.Canceled {
		return true
	}
	// grpc does not set cancel correctly when stream gets cancelled and then Recv is called
	if err != nil && ctx.Err() == context.Canceled {
		// when this error comes from containerd it is not typed at all, just concatenated string
		if strings.Contains(err.Error(), "EOF") {
			return true
		}
		if strings.Contains(err.Error(), context.Canceled.Error()) {
			return true
		}
	}
	return false
}
