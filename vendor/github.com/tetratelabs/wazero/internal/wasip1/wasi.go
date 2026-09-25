// Package wasip1 is a helper to remove package cycles re-using constants.
package wasip1

import (
	"strings"
)

// InternalModuleName is not named ModuleName, to avoid a clash on dot imports.
const InternalModuleName = "wasi_snapshot_preview1"

func flagsString(names []string, f int) string {
	var builder strings.Builder
	first := true
	for i, sf := range names {
		target := 1 << i
		if target&f != 0 {
			if !first {
				builder.WriteByte('|')
			} else {
				first = false
			}
			builder.WriteString(sf)
		}
	}
	return builder.String()
}
