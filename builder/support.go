package builder

import (
	"regexp"
	"strings"
)

var (
	// `\\\\+|[^\\]|\b|\A` - match any number of "\\" (ie, properly-escaped backslashes), or a single non-backslash character, or a word boundary, or beginning-of-line
	// `\$` - match literal $
	// `[[:alnum:]_]+` - match things like `$SOME_VAR`
	// `{[[:alnum:]_]+}` - match things like `${SOME_VAR}`
	tokenEnvInterpolation = regexp.MustCompile(`(\\|\\\\+|[^\\]|\b|\A)\$([[:alnum:]_]+|{[[:alnum:]_]+})`)
	// this intentionally punts on more exotic interpolations like ${SOME_VAR%suffix} and lets the shell handle those directly
)

// handle environment replacement. Used in dispatcher.
func (b *Builder) replaceEnv(str string) string {
	for _, match := range tokenEnvInterpolation.FindAllString(str, -1) {
		idx := strings.Index(match, "\\$")
		if idx != -1 {
			if idx+2 >= len(match) {
				str = strings.Replace(str, match, "\\$", -1)
				continue
			}

			prefix := match[:idx]
			stripped := match[idx+2:]
			str = strings.Replace(str, match, prefix+"$"+stripped, -1)
			continue
		}

		match = match[strings.Index(match, "$"):]
		matchKey := strings.Trim(match, "${}")

		for _, keyval := range b.Config.Env {
			tmp := strings.SplitN(keyval, "=", 2)
			if tmp[0] == matchKey {
				str = strings.Replace(str, match, tmp[1], -1)
				break
			}
		}
	}

	return str
}

func handleJsonArgs(args []string, attributes map[string]bool) []string {
	if len(args) == 0 {
		return []string{}
	}

	if attributes != nil && attributes["json"] {
		return args
	}

	// literal string command, not an exec array
	return []string{strings.Join(args, " ")}
}
