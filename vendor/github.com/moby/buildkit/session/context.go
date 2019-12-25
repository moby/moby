package session

import "context"

type contextKeyT string

var contextKey = contextKeyT("buildkit/session-id")

func NewContext(ctx context.Context, id string) context.Context {
	if id != "" {
		return context.WithValue(ctx, contextKey, id)
	}
	return ctx
}

func FromContext(ctx context.Context) string {
	v := ctx.Value(contextKey)
	if v == nil {
		return ""
	}
	return v.(string)
}
