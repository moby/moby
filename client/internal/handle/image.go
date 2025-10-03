package internalhandle

import "context"

type ImageResolveResult struct {
	RefOrTruncatedID string
}

type naiveHandle struct {
	RefOrTruncatedID string
}

func (h *naiveHandle) Resolve(ctx context.Context) (ImageResolveResult, error) {
	return ImageResolveResult{RefOrTruncatedID: h.RefOrTruncatedID}, nil
}
