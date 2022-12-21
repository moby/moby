package parser

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

const (
	keySyntax = "syntax"
	keyEscape = "escape"
)

var validDirectives = map[string]struct{}{
	keySyntax: {},
	keyEscape: {},
}

type Directive struct {
	Name     string
	Value    string
	Location []Range
}

// DirectiveParser is a parser for Dockerfile directives that enforces the
// quirks of the directive parser.
type DirectiveParser struct {
	line   int
	regexp *regexp.Regexp
	seen   map[string]struct{}
	done   bool
}

func (d *DirectiveParser) setComment(comment string) {
	d.regexp = regexp.MustCompile(fmt.Sprintf(`^%s\s*([a-zA-Z][a-zA-Z0-9]*)\s*=\s*(.+?)\s*$`, comment))
}

func (d *DirectiveParser) ParseLine(line []byte) (*Directive, error) {
	d.line++
	if d.done {
		return nil, nil
	}
	if d.regexp == nil {
		d.setComment("#")
	}

	match := d.regexp.FindSubmatch(line)
	if len(match) == 0 {
		d.done = true
		return nil, nil
	}

	k := strings.ToLower(string(match[1]))
	if _, ok := validDirectives[k]; !ok {
		d.done = true
		return nil, nil
	}
	if d.seen == nil {
		d.seen = map[string]struct{}{}
	}
	if _, ok := d.seen[k]; ok {
		return nil, errors.Errorf("only one %s parser directive can be used", k)
	}
	d.seen[k] = struct{}{}

	v := string(match[2])

	directive := Directive{
		Name:  k,
		Value: v,
		Location: []Range{{
			Start: Position{Line: d.line},
			End:   Position{Line: d.line},
		}},
	}
	return &directive, nil
}

func (d *DirectiveParser) ParseAll(data []byte) ([]*Directive, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var directives []*Directive
	for scanner.Scan() {
		if d.done {
			break
		}

		d, err := d.ParseLine(scanner.Bytes())
		if err != nil {
			return directives, err
		}
		if d != nil {
			directives = append(directives, d)
		}
	}
	return directives, nil
}

// DetectSyntax returns the syntax of provided input.
//
// The traditional dockerfile directives '# syntax = ...' are used by default,
// however, the function will also fallback to c-style directives '// syntax = ...'
// and json-encoded directives '{ "syntax": "..." }'. Finally, starting lines
// with '#!' are treated as shebangs and ignored.
//
// This allows for a flexible range of input formats, and appropriate syntax
// selection.
func DetectSyntax(dt []byte) (string, string, []Range, bool) {
	dt, hadShebang, err := discardShebang(dt)
	if err != nil {
		return "", "", nil, false
	}
	line := 0
	if hadShebang {
		line++
	}

	// use default directive parser, and search for #syntax=
	directiveParser := DirectiveParser{line: line}
	if syntax, cmdline, loc, ok := detectSyntaxFromParser(dt, directiveParser); ok {
		return syntax, cmdline, loc, true
	}

	// use directive with different comment prefix, and search for //syntax=
	directiveParser = DirectiveParser{line: line}
	directiveParser.setComment("//")
	if syntax, cmdline, loc, ok := detectSyntaxFromParser(dt, directiveParser); ok {
		return syntax, cmdline, loc, true
	}

	// search for possible json directives
	var directive struct {
		Syntax string `json:"syntax"`
	}
	if err := json.Unmarshal(dt, &directive); err == nil {
		if directive.Syntax != "" {
			loc := []Range{{
				Start: Position{Line: line},
				End:   Position{Line: line},
			}}
			return directive.Syntax, directive.Syntax, loc, true
		}
	}

	return "", "", nil, false
}

func detectSyntaxFromParser(dt []byte, parser DirectiveParser) (string, string, []Range, bool) {
	directives, _ := parser.ParseAll(dt)
	for _, d := range directives {
		// check for syntax directive before erroring out, since the error
		// might have occurred *after* the syntax directive
		if d.Name == keySyntax {
			p, _, _ := strings.Cut(d.Value, " ")
			return p, d.Value, d.Location, true
		}
	}
	return "", "", nil, false
}

func discardShebang(dt []byte) ([]byte, bool, error) {
	line, rest, _ := bytes.Cut(dt, []byte("\n"))
	if bytes.HasPrefix(line, []byte("#!")) {
		return rest, true, nil
	}
	return dt, false, nil
}
