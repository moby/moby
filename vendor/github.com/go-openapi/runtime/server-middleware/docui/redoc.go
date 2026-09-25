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

// UseRedoc creates a middleware to serve a documentation site for a swagger spec using [Redoc].
//
// [Redoc]: https://redocly.com/docs/redoc
func UseRedoc(opts ...Option) func(next http.Handler) http.Handler {
	pth, assets := redocSetup(opts)

	return func(next http.Handler) http.Handler {
		return serveUI(pth, assets, next)
	}
}

// Redoc creates a [http.Handler] to serve a documentation site for a swagger spec using [Redoc].
//
// By default, the UI is served at route "/docs"
//
// This allows for altering the spec before starting the [http] listener.
//
// [Redoc]: https://redocly.com/docs/redoc
func Redoc(next http.Handler, opts ...Option) http.Handler {
	pth, assets := redocSetup(opts)

	return serveUI(pth, assets, next)
}

func redocSetup(opts []Option) (pth string, assets []byte) {
	o := optionsWithDefaults(opts,
		// defaults for redoc
		WithUITemplate(redocTemplate),
		WithUIAssetsURL(redocLatest),
	)

	pth = path.Join(o.BasePath, o.Path)
	tmpl := template.Must(template.New("redoc").Parse(o.Template))
	buf := bytes.NewBuffer(nil)
	if err := tmpl.Execute(buf, o); err != nil {
		panic(fmt.Errorf("cannot execute template: %w", err))
	}

	return pth, buf.Bytes()
}

const (
	redocLatest   = "https://cdn.redoc.ly/redoc/latest/bundles/redoc.standalone.js" // "https://cdn.jsdelivr.net/npm/redoc/bundles/redoc.standalone.js"
	redocTemplate = `<!DOCTYPE html>
<html>
  <head>
    <title>{{ .Title }}</title>
		<!-- needed for adaptive design -->
		<meta charset="utf-8"/>
		<meta name="viewport" content="width=device-width, initial-scale=1">
		<link href="https://fonts.googleapis.com/css?family=Montserrat:300,400,700|Roboto:300,400,700" rel="stylesheet">

    <!--
    ReDoc doesn't change outer page styles
    -->
    <style>
      body {
        margin: 0;
        padding: 0;
      }
    </style>
  </head>
  <body>
    <redoc spec-url='{{ .SpecURL }}'></redoc>
    <script src="{{ .AssetsURL }}"> </script>
  </body>
</html>
`
)
