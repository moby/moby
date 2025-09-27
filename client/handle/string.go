package handle

import (
	"context"
	"errors"

	internalhandle "github.com/moby/moby/client/internal/handle"
)

func FromString(str string) *stringHandle {
	return &stringHandle{str: str}
}

type stringHandle struct {
	str string
}

func (h *stringHandle) ResolveImage(ctx context.Context) (internalhandle.ImageResolveResult, error) {
	if h.str == "" {
		return internalhandle.ImageResolveResult{}, errors.New("empty image name")
	}
	return internalhandle.ImageResolveResult{RefOrTruncatedID: h.str}, nil
}
