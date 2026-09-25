// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package docui

import (
	"fmt"
	"net/http"
	"path"
)

// serveUI creates a [http.Handler] that serves a templated asset as text/html.
func serveUI(pth string, assets []byte, next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if path.Clean(r.URL.Path) == pth {
			rw.Header().Set(contentTypeHeader, "text/html; charset=utf-8")
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write(assets)

			return
		}

		if next != nil {
			next.ServeHTTP(rw, r)

			return
		}

		rw.Header().Set(contentTypeHeader, "text/plain")
		rw.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(rw, "%q not found", pth)
	})
}
