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

// UseSwaggerUI creates a middleware to serve a documentation site for a swagger spec using [SwaggerUI].
//
// [SwaggerUI]: https://swagger.io/tools/swagger-ui
func UseSwaggerUI(opts ...Option) func(next http.Handler) http.Handler {
	pth, assets := swaggeruiSetup(opts)

	return func(next http.Handler) http.Handler {
		return serveUI(pth, assets, next)
	}
}

// SwaggerUI creates a [http.Handler] to serve a documentation site for a swagger spec using [SwaggerUI].
//
// By default, the UI is served at route "/docs"
//
// This allows for altering the spec before starting the [http] listener.
//
// [SwaggerUI]: https://swagger.io/tools/swagger-ui
func SwaggerUI(next http.Handler, opts ...Option) http.Handler {
	pth, assets := swaggeruiSetup(opts)

	return serveUI(pth, assets, next)
}

func swaggeruiSetup(opts []Option) (pth string, assets []byte) {
	o := optionsWithDefaults(opts,
		// defaults for SwaggerUI
		WithUITemplate(swaggeruiTemplate),
		WithUIAssetsURL(swaggerLatest),
	)
	o.applySwaggerUIDefaults()
	if o.OAuth2CallbackURL == "" {
		o.OAuth2CallbackURL = path.Join(o.BasePath, o.Path, "oauth2-callback")
	}

	pth = path.Join(o.BasePath, o.Path)
	tmpl := template.Must(template.New("swaggerui").Parse(o.Template))
	buf := bytes.NewBuffer(nil)
	if err := tmpl.Execute(buf, o); err != nil {
		panic(fmt.Errorf("cannot execute template: %w", err))
	}

	return pth, buf.Bytes()
}

const (
	swaggerLatest          = "https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js"
	swaggerPresetLatest    = "https://unpkg.com/swagger-ui-dist/swagger-ui-standalone-preset.js"
	swaggerStylesLatest    = "https://unpkg.com/swagger-ui-dist/swagger-ui.css"
	swaggerFavicon32Latest = "https://unpkg.com/swagger-ui-dist/favicon-32x32.png"
	swaggerFavicon16Latest = "https://unpkg.com/swagger-ui-dist/favicon-16x16.png"
	swaggeruiTemplate      = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8">
		<title>{{ .Title }}</title>

	  {{- if .SwaggerStylesURL }}
    <link rel="stylesheet" type="text/css" href="{{ .SwaggerStylesURL }}" />
	  {{- end }}
	  {{- if .Favicon32 }}
    <link rel="icon" type="image/png" href="{{ .Favicon32 }}" sizes="32x32" />
	  {{- end }}
	  {{- if .Favicon16 }}
    <link rel="icon" type="image/png" href="{{ .Favicon16 }}" sizes="16x16" />
	  {{- end }}
    <style>
      html
      {
        box-sizing: border-box;
        overflow: -moz-scrollbars-vertical;
        overflow-y: scroll;
      }

      *,
      *:before,
      *:after
      {
        box-sizing: inherit;
      }

      body
      {
        margin:0;
        background: #fafafa;
      }
    </style>
  </head>

  <body>
    <div id="swagger-ui"></div>

    <script src="{{ .AssetsURL }}"> </script>
	  {{- if .SwaggerPresetURL }}
    <script src="{{ .SwaggerPresetURL }}"> </script>
	  {{- end }}
    <script>
    window.onload = function() {
      // Begin Swagger UI call region
      const ui = SwaggerUIBundle({
        url: '{{ .SpecURL }}',
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        plugins: [
          SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: "StandaloneLayout",
	      {{- if .OAuth2CallbackURL }}
        oauth2RedirectUrl: '{{ .OAuth2CallbackURL }}'
	      {{- end }}
      })
      // End Swagger UI call region

      window.ui = ui
    }
  </script>
  </body>
</html>
`
)
