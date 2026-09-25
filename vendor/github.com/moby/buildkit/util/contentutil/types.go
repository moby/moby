package contentutil

import (
	"context"

	"github.com/containerd/containerd/v2/core/remotes"
	intoto "github.com/in-toto/in-toto-golang/in_toto"
)

// RegisterContentPayloadTypes registers content types that are not defined by
// default but that we expect to find in registry images.
func RegisterContentPayloadTypes(ctx context.Context) context.Context {
	ctx = remotes.WithMediaTypeKeyPrefix(ctx, intoto.PayloadType, "intoto")
	return ctx
}
