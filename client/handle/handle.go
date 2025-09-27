package handle

import (
	"context"

	internalhandle "github.com/moby/moby/client/internal/handle"
)

type ImageHandle interface {
	ResolveImage(ctx context.Context) (internalhandle.ImageResolveResult, error)
}
