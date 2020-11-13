package winlayers

import "context"

type contextKeyT string

var contextKey = contextKeyT("buildkit/winlayers-on")

func UseWindowsLayerMode(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey, true)
}

func hasWindowsLayerMode(ctx context.Context) bool {
	v := ctx.Value(contextKey)
	if v == nil {
		return false
	}
	return true
}
