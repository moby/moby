package evaluator

import (
	"regexp"
	"strings"
)

var (
	TOKEN_ENV_INTERPOLATION = regexp.MustCompile("(\\\\\\\\+|[^\\\\]|\\b|\\A)\\$({?)([[:alnum:]_]+)(}?)")
)

func replaceEnv(b *buildFile, str string) string {
	for _, match := range TOKEN_ENV_INTERPOLATION.FindAllString(str, -1) {
		match = match[strings.Index(match, "$"):]
		matchKey := strings.Trim(match, "${}")

		for envKey, envValue := range b.env {
			if matchKey == envKey {
				str = strings.Replace(str, match, envValue, -1)
			}
		}
	}

	return str
}
