// +build go1.13

package cmp

import (
	"errors"
)

// ErrorIs succeeds if errors.Is(actual, expected) returns true. See
// https://golang.org/pkg/errors/#Is for accepted argument values.
func ErrorIs(actual error, expected error) Comparison {
	return func() Result {
		if errors.Is(actual, expected) {
			return ResultSuccess
		}

		return ResultFailureTemplate(`error is
			{{- if not .Data.a }} nil,{{ else }}
			{{- printf " \"%v\"" .Data.a}} (
				{{- with callArg 0 }}{{ formatNode . }} {{end -}}
				{{- printf "%T" .Data.a -}}
			),
			{{- end }} not {{ printf "\"%v\"" .Data.x}} (
				{{- with callArg 1 }}{{ formatNode . }} {{end -}}
				{{- printf "%T" .Data.x -}}
			)`,
			map[string]interface{}{"a": actual, "x": expected})
	}
}
