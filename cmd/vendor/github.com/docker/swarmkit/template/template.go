package template

import (
	"strings"
	"text/template"
)

// funcMap defines functions for our template system.
var funcMap = template.FuncMap{
	"join": func(s ...string) string {
		// first arg is sep, remaining args are strings to join
		return strings.Join(s[1:], s[0])
	},
}

func newTemplate(s string) (*template.Template, error) {
	return template.New("expansion").Option("missingkey=error").Funcs(funcMap).Parse(s)
}
