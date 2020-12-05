package errdefs

import (
	"context"
	"errors"

	"github.com/moby/buildkit/util/grpcerrors"
	"google.golang.org/grpc/codes"
)

func IsCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || grpcerrors.Code(err) == codes.Canceled
}
