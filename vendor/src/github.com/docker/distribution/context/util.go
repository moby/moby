package context

import (
	"time"
)

// Since looks up key, which should be a time.Time, and returns the duration
// since that time. If the key is not found, the value returned will be zero.
// This is helpful when inferring metrics related to context execution times.
func Since(ctx Context, key interface{}) time.Duration {
	startedAtI := ctx.Value(key)
	if startedAtI != nil {
		if startedAt, ok := startedAtI.(time.Time); ok {
			return time.Since(startedAt)
		}
	}

	return 0
}

// GetStringValue returns a string value from the context. The empty string
// will be returned if not found.
func GetStringValue(ctx Context, key interface{}) (value string) {
	stringi := ctx.Value(key)
	if stringi != nil {
		if valuev, ok := stringi.(string); ok {
			value = valuev
		}
	}

	return value
}
