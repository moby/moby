package evaluator

import (
	"regexp"
	"strings"
)

var (
	TOKEN_ESCAPED_QUOTE     = regexp.MustCompile(`\\"`)
	TOKEN_ESCAPED_ESCAPE    = regexp.MustCompile(`\\\\`)
	TOKEN_ENV_INTERPOLATION = regexp.MustCompile("(\\\\\\\\+|[^\\\\]|\\b|\\A)\\$({?)([[:alnum:]_]+)(}?)")
)

func stripQuotes(str string) string {
	str = str[1 : len(str)-1]
	str = TOKEN_ESCAPED_QUOTE.ReplaceAllString(str, `"`)
	return TOKEN_ESCAPED_ESCAPE.ReplaceAllString(str, `\`)
}

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
