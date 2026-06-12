// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build openapi_unsafe_skipauth

package middleware

import (
	"log"
	"net/http"
	"sync/atomic"
)

// skipAuthEnabled holds the process-wide skip-auth flag. It only
// exists in binaries built with the `openapi_unsafe_skipauth` tag —
// production binaries (built without the tag) have no field, no
// setter, no storage, and no skip-checking branch in [Context.Authorize].
// Reflection, unsafe-pointer arithmetic, or a debugger cannot flip
// what is not in the binary.
var skipAuthEnabled atomic.Bool

// SetSkipAuth toggles a PROCESS-WIDE bypass of authentication AND
// authorization for every operation served by every Context in the
// running program.
//
// DANGER: this disables ALL authentication and ALL authorization.
// Every request to every secured endpoint runs as if it had been
// authorized with a nil principal. Use ONLY on developer
// workstations during early prototyping (e.g. while
// authentication is not yet wired up).
//
// This function exists only when the build tag
// `openapi_unsafe_skipauth` is set:
//
//	go build -tags openapi_unsafe_skipauth ./...
//
// Production CI MUST NOT pass this tag. Calls compile to a symbol
// that does not exist in production binaries.
//
// Calling with true emits a one-line WARNING via the stdlib `log`
// package (stderr by default) so the bypass is visible at startup.
// Calling with false silently disables it.
func SetSkipAuth(skip bool) {
	skipAuthEnabled.Store(skip)
	if skip {
		log.Println("WARNING: go-openapi/runtime: SetSkipAuth(true) — authentication and authorization are bypassed for ALL operations. This MUST NOT run in production.")
	}
}

// Authorize is the dev-build variant of the production
// [Context.Authorize] (see context_skipauth_disabled.go for the
// production path). When [SetSkipAuth] has enabled the bypass, this
// returns a nil principal with the original request and no error —
// handlers downstream receive a nil-value principal. Otherwise it
// delegates to the standard authentication+authorization body.
func (c *Context) Authorize(request *http.Request, route *MatchedRoute) (any, *http.Request, error) {
	if skipAuthEnabled.Load() {
		return nil, request, nil
	}
	return c.authorizeImpl(request, route)
}
