// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package docui provides standalone HTTP middlewares that serve OpenAPI
// documentation UIs (Swagger UI, ReDoc, RapiDoc) and the spec document
// itself.
//
// The package is stdlib-only and has no transitive dependency on any
// OpenAPI spec, loading or validation library, so it may be imported by
// any net/http application that simply wants to mount a documentation
// site.
package docui
