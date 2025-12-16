// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package middleware

import "net/http"

func newSecureAPI(ctx *Context, next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		route, rCtx, _ := ctx.RouteInfo(r)
		if rCtx != nil {
			r = rCtx
		}
		if route != nil && !route.NeedsAuth() {
			next.ServeHTTP(rw, r)
			return
		}

		_, rCtx, err := ctx.Authorize(r, route)
		if err != nil {
			ctx.Respond(rw, r, route.Produces, route, err)
			return
		}
		r = rCtx

		next.ServeHTTP(rw, r)
	})
}
