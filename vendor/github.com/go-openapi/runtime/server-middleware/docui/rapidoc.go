// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package docui

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"path"
)

// UseRapiDoc creates a middleware to serve a documentation site for a swagger spec using [RapidDoc].
//
// [RapiDoc]: https://github.com/rapi-doc/RapiDoc
func UseRapiDoc(opts ...Option) func(next http.Handler) http.Handler {
	pth, assets := rapiDocSetup(opts)
	return func(next http.Handler) http.Handler {
		return serveUI(pth, assets, next)
	}
}

// RapiDoc creates a [http.Handler] to serve a documentation site for a swagger spec using [RapidDoc].
//
// By default, the UI is served at route "/docs"
//
// This allows for altering the spec before starting the [http] listener.
//
// [RapiDoc]: https://github.com/rapi-doc/RapiDoc
func RapiDoc(next http.Handler, opts ...Option) http.Handler {
	pth, assets := rapiDocSetup(opts)

	return serveUI(pth, assets, next)
}

func rapiDocSetup(opts []Option) (pth string, assets []byte) {
	o := optionsWithDefaults(opts,
		// defaults for rapiDoc
		WithUITemplate(rapidocTemplate),
		WithUIAssetsURL(rapidocLatest),
	)
	pth = path.Join(o.BasePath, o.Path)
	tmpl := template.Must(template.New("rapidoc").Parse(o.Template))
	buf := bytes.NewBuffer(nil)
	if err := tmpl.Execute(buf, o); err != nil {
		panic(fmt.Errorf("cannot execute template: %w", err))
	}

	return pth, buf.Bytes()
}

const (
	rapidocLatest   = "https://unpkg.com/rapidoc/dist/rapidoc-min.js"
	rapidocTemplate = `<!doctype html>
<html>
<head>
  <title>{{ .Title }}</title>
  <meta charset="utf-8"> <!-- Important: rapi-doc uses utf8 characters -->
  <script type="module" src="{{ .AssetsURL }}"></script>
</head>
<body>
  <rapi-doc spec-url="{{ .SpecURL }}"></rapi-doc>
</body>
</html>
`
)
