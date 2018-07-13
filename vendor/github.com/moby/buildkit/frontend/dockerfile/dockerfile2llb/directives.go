package dockerfile2llb

import (
	"bufio"
	"io"
	"regexp"
	"strings"
)

const keySyntax = "syntax"

var reDirective = regexp.MustCompile(`^#\s*([a-zA-Z][a-zA-Z0-9]*)\s*=\s*(.+?)\s*$`)

func DetectSyntax(r io.Reader) (string, string, bool) {
	directives := ParseDirectives(r)
	if len(directives) == 0 {
		return "", "", false
	}
	v, ok := directives[keySyntax]
	if !ok {
		return "", "", false
	}
	p := strings.SplitN(v, " ", 2)
	return p[0], v, true
}

func ParseDirectives(r io.Reader) map[string]string {
	m := map[string]string{}
	s := bufio.NewScanner(r)
	for s.Scan() {
		match := reDirective.FindStringSubmatch(s.Text())
		if len(match) == 0 {
			return m
		}
		m[strings.ToLower(match[1])] = match[2]
	}
	return m
}
