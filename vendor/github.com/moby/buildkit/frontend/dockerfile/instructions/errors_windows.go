package instructions

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

func errNotJSON(command, original string) error {
	// For Windows users, give a hint if it looks like it might contain
	// a path which hasn't been escaped such as ["c:\windows\system32\prog.exe", "-param"],
	// as JSON must be escaped. Unfortunate...
	//
	// Specifically looking for quote-driveletter-colon-backslash, there's no
	// double backslash and a [] pair. No, this is not perfect, but it doesn't
	// have to be. It's simply a hint to make life a little easier.
	extra := ""
	original = filepath.FromSlash(strings.ToLower(strings.Replace(strings.ToLower(original), strings.ToLower(command)+" ", "", -1)))
	if len(regexp.MustCompile(`"[a-z]:\\.*`).FindStringSubmatch(original)) > 0 &&
		!strings.Contains(original, `\\`) &&
		strings.Contains(original, "[") &&
		strings.Contains(original, "]") {
		extra = fmt.Sprintf(`. It looks like '%s' includes a file path without an escaped back-slash. JSON requires back-slashes to be escaped such as ["c:\\path\\to\\file.exe", "/parameter"]`, original)
	}
	return fmt.Errorf("%s requires the arguments to be in JSON form%s", command, extra)
}
