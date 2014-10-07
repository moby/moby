package builder

import (
	"regexp"
	"strings"
)

var (
	TOKEN_ENV_INTERPOLATION = regexp.MustCompile(`(\\\\+|[^\\]|\b|\A)\$({?)([[:alnum:]_]+)(}?)`)
)

// handle environment replacement. Used in dispatcher.
func (b *Builder) replaceEnv(str string) string {
	for _, match := range TOKEN_ENV_INTERPOLATION.FindAllString(str, -1) {
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
	if attributes["json"] {
		return args
	}

	// HACK: in certain scenarios, [] will get transformed to [""]. This needs to
	// be fixed in the parser, which requires larger effort to fix. At the time of
	// this writing, it was felt that the easier fix was better unless the more
	// fundamental issue can be addressed.
	if len(args) == 0 {
		return []string{}
	}

	return []string{strings.Join(args, " ")}
}
