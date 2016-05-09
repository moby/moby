package shellwords

import (
	"errors"
	"os"
	"regexp"
	"strings"
)

var (
	ParseEnv      bool = false
	ParseBacktick bool = false
)

var envRe = regexp.MustCompile(`\$({[a-zA-Z0-9_]+}|[a-zA-Z0-9_]+)`)

func isSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\r', '\n':
		return true
	}
	return false
}

func replaceEnv(s string) string {
	return envRe.ReplaceAllStringFunc(s, func(s string) string {
		s = s[1:]
		if s[0] == '{' {
			s = s[1 : len(s)-1]
		}
		return os.Getenv(s)
	})
}

type Parser struct {
	ParseEnv      bool
	ParseBacktick bool
}

func NewParser() *Parser {
	return &Parser{ParseEnv, ParseBacktick}
}

func (p *Parser) Parse(line string) ([]string, error) {
	line = strings.TrimSpace(line)

	args := []string{}
	buf := ""
	var escaped, doubleQuoted, singleQuoted, backQuote bool
	backtick := ""

	for _, r := range line {
		if escaped {
			buf += string(r)
			escaped = false
			continue
		}

		if r == '\\' {
			if singleQuoted {
				buf += string(r)
			} else {
				escaped = true
			}
			continue
		}

		if isSpace(r) {
			if singleQuoted || doubleQuoted || backQuote {
				buf += string(r)
				backtick += string(r)
			} else if buf != "" {
				if p.ParseEnv {
					buf = replaceEnv(buf)
				}
				args = append(args, buf)
				buf = ""
			}
			continue
		}

		switch r {
		case '`':
			if !singleQuoted && !doubleQuoted {
				if p.ParseBacktick {
					if backQuote {
						out, err := shellRun(backtick)
						if err != nil {
							return nil, err
						}
						buf = out
					}
					backtick = ""
					backQuote = !backQuote
					continue
				}
				backtick = ""
				backQuote = !backQuote
			}
		case '"':
			if !singleQuoted {
				doubleQuoted = !doubleQuoted
				continue
			}
		case '\'':
			if !doubleQuoted {
				singleQuoted = !singleQuoted
				continue
			}
		}

		buf += string(r)
		if backQuote {
			backtick += string(r)
		}
	}

	if buf != "" {
		if p.ParseEnv {
			buf = replaceEnv(buf)
		}
		args = append(args, buf)
	}

	if escaped || singleQuoted || doubleQuoted || backQuote {
		return nil, errors.New("invalid command line string")
	}

	return args, nil
}

func Parse(line string) ([]string, error) {
	return NewParser().Parse(line)
}
