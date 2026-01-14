// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package middleware

import "net/http"

// NewOperationExecutor creates a context aware middleware that handles the operations after routing
func NewOperationExecutor(ctx *Context) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// use context to lookup routes
		route, rCtx, _ := ctx.RouteInfo(r)
		if rCtx != nil {
			r = rCtx
		}

		route.Handler.ServeHTTP(rw, r)
	})
}
