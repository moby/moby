package dockerfile

import (
	"fmt"
	"regexp"
	"strings"
)

type Handler interface {
	Handle(stepname, cmd, arg string) error
}

// Long lines can be split with a backslash
var lineContinuation = regexp.MustCompile(`\s*\\\s*\n`)

func stripComments(raw []byte) string {
	var (
		out   []string
		lines = strings.Split(string(raw), "\n")
	)
	for _, l := range lines {
		if len(l) == 0 || l[0] == '#' {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

func ParseScript(src []byte, handler Handler) error {
	var (
		ppSrc = lineContinuation.ReplaceAllString(stripComments(src), "")
		stepN = 0
	)
	for _, line := range strings.Split(ppSrc, "\n") {
		line = strings.Trim(strings.Replace(line, "\t", " ", -1), " \t\r\n")
		if len(line) == 0 {
			continue
		}
		if err := ParseExpr(fmt.Sprintf("%d", stepN), line, handler); err != nil {
			return fmt.Errorf("%s: %v", line, err)
		}
		stepN += 1
	}
	return nil

}

func ParseExpr(n string, expression string, handler Handler) error {
	tmp := strings.SplitN(expression, " ", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("invalid format %s", expression)
	}
	instruction := strings.ToLower(strings.Trim(tmp[0], " "))
	if len(instruction) == 0 {
		return fmt.Errorf("invalid format: %s", expression)
	}
	arg := strings.Trim(tmp[1], " ")
	if handler == nil {
		return nil
	}
	return handler.Handle(n, instruction, arg)
}
