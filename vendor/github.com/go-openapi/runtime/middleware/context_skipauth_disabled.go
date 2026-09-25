// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build !openapi_unsafe_skipauth

package middleware

import "net/http"

// Authorize authorizes the request.
//
// Returns the principal object and a shallow copy of the request when its
// context doesn't contain the principal, otherwise the same request or an error
// (the last) if one of the authenticators returns one or an Unauthenticated error.
//
// This is the production variant — compiled when the build tag
// `openapi_unsafe_skipauth` is NOT set. There is no skip-auth check
// in this codepath; the field, setter, and storage for the bypass
// flag are entirely absent from the binary. See the alternate
// implementation in context_skipauth_enabled.go for the dev-only
// bypass mechanism.
func (c *Context) Authorize(request *http.Request, route *MatchedRoute) (any, *http.Request, error) {
	return c.authorizeImpl(request, route)
}
