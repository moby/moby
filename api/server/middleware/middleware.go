package middleware

import "github.com/docker/docker/api/server/httputils"

// Middleware is an adapter to allow the use of ordinary functions as Docker API filters.
// Any function that has the appropriate signature can be register as a middleware.
type Middleware func(handler httputils.APIFunc) httputils.APIFunc
