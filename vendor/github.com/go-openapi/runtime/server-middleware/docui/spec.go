// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package docui

import (
	"net/http"
	"path"
)

// UseSpec creates a middleware to serve a swagger spec as a JSON document.
func UseSpec(spec []byte, opts ...SpecOption) func(next http.Handler) http.Handler {
	o := specOptionsWithDefaults(opts)

	return func(next http.Handler) http.Handler {
		return handleSpec(o.Path, spec, next)
	}
}

// ServeSpec creates a [http.Handler] to serve a swagger spec as a JSON document.
//
// This allows for altering the spec before starting the [http] listener.
//
// Additional [SpecOption] can be used to change the path and the name of the document (defaults to "/swagger.json").
func ServeSpec(spec []byte, next http.Handler, opts ...SpecOption) http.Handler {
	o := specOptionsWithDefaults(opts)

	return handleSpec(o.Path, spec, next)
}

func handleSpec(pth string, spec []byte, next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if path.Clean(r.URL.Path) == pth {
			rw.Header().Set(contentTypeHeader, applicationJSON)
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write(spec)

			return
		}

		if next != nil {
			next.ServeHTTP(rw, r)

			return
		}

		rw.Header().Set(contentTypeHeader, applicationJSON)
		rw.WriteHeader(http.StatusNotFound)
	})
}
