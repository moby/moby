package pathutil

import (
	"path/filepath"
	"strings"
	"unicode"
)

func SafeFileName(s string) string {
	defaultName := "download"
	name := filepath.Base(filepath.FromSlash(strings.TrimSpace(s)))
	if name == "" || name == "." || name == ".." {
		return defaultName
	}
	for _, r := range name {
		if r == 0 || unicode.IsControl(r) {
			return defaultName
		}
	}
	return name
}
