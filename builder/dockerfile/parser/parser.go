// Package parser implements a parser and parse tree dumper for Dockerfiles.
package parser

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"

	"github.com/docker/docker/builder/dockerfile/command"
)

// Node is a structure used to represent a parse tree.
//
// In the node there are three fields, Value, Next, and Children. Value is the
// current token's string value. Next is always the next non-child token, and
// children contains all the children. Here's an example:
//
// (value next (child child-next child-next-next) next-next)
//
// This data structure is frankly pretty lousy for handling complex languages,
// but lucky for us the Dockerfile isn't very complicated. This structure
// works a little more effectively than a "proper" parse tree for our needs.
//
type Node struct {
	Value      string          // actual content
	Next       *Node           // the next item in the current sexp
	Children   []*Node         // the children of this sexp
	Attributes map[string]bool // special attributes for this node
	Original   string          // original line used before parsing
	Flags      []string        // only top Node should have this set
	startLine  int             // the line in the original dockerfile where the node begins (used in tests)
	endLine    int             // the line in the original dockerfile where the node ends (used in tests)
}

// Directives is the structure used during a build run to hold the state of
// parsing directives.
type Directives struct {
	EscapeToken           rune                // Current escape token
	LineContinuationRegex *regexp.Regexp      // Current line contination regex
	usedDirectives        map[string]struct{} // Whether a directive has been seen
}

var (
	dispatch        map[string]func(string, Directives) (*Node, map[string]bool, error)
	tokenWhitespace = regexp.MustCompile(`[\t\v\f\r ]+`)
	directiveRegexp = regexp.MustCompile(`^#[ \t]*([a-z0-9]+)[ \t]*=[ \t]*(.*)$`)
	tokenComment    = regexp.MustCompile(`^#.*$`)
	errNoDirective  = errors.New("not a directive")
)

// DefaultEscapeToken is the default escape token
const DefaultEscapeToken = "\\"

// DefaultDirectives returns directives struct with default properties
func DefaultDirectives() Directives {
	d := &Directives{usedDirectives: make(map[string]struct{})}
	if err := d.SetEscapeToken(DefaultEscapeToken); err != nil {
		panic(err)
	}
	return *d
}

// SetEscapeToken sets the default token for escaping characters in a Dockerfile.
func (d *Directives) SetEscapeToken(s string) error {
	if s != "`" && s != "\\" {
		return fmt.Errorf("invalid ESCAPE '%s'. Must be ` or \\", s)
	}
	d.EscapeToken = rune(s[0])
	d.LineContinuationRegex = regexp.MustCompile(`\` + s + `$`)
	return nil
}

func init() {
	// Dispatch Table. see line_parsers.go for the parse functions.
	// The command is parsed and mapped to the line parser. The line parser
	// receives the arguments but not the command, and returns an AST after
	// reformulating the arguments according to the rules in the parser
	// functions. Errors are propagated up by Parse() and the resulting AST can
	// be incorporated directly into the existing AST as a next.
	dispatch = map[string]func(string, Directives) (*Node, map[string]bool, error){
		command.Add:         parseMaybeJSONToList,
		command.Arg:         parseNameOrNameVal,
		command.Cmd:         parseMaybeJSON,
		command.Copy:        parseMaybeJSONToList,
		command.Entrypoint:  parseMaybeJSON,
		command.Env:         parseEnv,
		command.Expose:      parseStringsWhitespaceDelimited,
		command.From:        parseString,
		command.Healthcheck: parseHealthConfig,
		command.Label:       parseLabel,
		command.Maintainer:  parseString,
		command.Onbuild:     parseSubCommand,
		command.Run:         parseMaybeJSON,
		command.Shell:       parseMaybeJSON,
		command.StopSignal:  parseString,
		command.User:        parseString,
		command.Volume:      parseMaybeJSONToList,
		command.Workdir:     parseString,
	}
}

// ParseLine parses a line and returns the remainder.
func ParseLine(line string, d Directives) (string, *Node, error) {
	if line = stripComments(line); line == "" {
		return "", nil, nil
	}
	if d.LineContinuationRegex.MatchString(line) {
		line = d.LineContinuationRegex.ReplaceAllString(line, "")
		return line, nil, nil
	}

	cmd, flags, args, err := splitCommand(line)
	if err != nil {
		return "", nil, err
	}

	node := &Node{}
	node.Value = cmd

	sexp, attrs, err := fullDispatch(cmd, args, d)
	if err != nil {
		return "", nil, err
	}

	node.Next = sexp
	node.Attributes = attrs
	node.Original = line
	node.Flags = flags

	return "", node, nil
}

// ParseDirectives attempts to parse a directive from input line
func ParseDirectives(line string, d *Directives) (err error) {
	tecMatch := directiveRegexp.FindStringSubmatch(strings.ToLower(line))
	if len(tecMatch) > 2 {
		if _, ok := d.usedDirectives[tecMatch[1]]; ok {
			return fmt.Errorf("only one %v parser directive can be used", tecMatch[1])
		}
		defer func() {
			if err != nil {
				d.usedDirectives[tecMatch[1]] = struct{}{}
			}
		}()
		switch tecMatch[1] {
		case "escape":
			return d.SetEscapeToken(tecMatch[2])
		}
	}
	return errNoDirective
}

// Parse is the main parse routine.
// It handles an io.ReadWriteCloser and returns the root of the AST.
func Parse(r io.Reader) (*Node, error) {
	startLine, currentLine := 0, 0
	parseDirectives := true
	root := &Node{}
	root.startLine = -1
	scanner := bufio.NewScanner(r)
	d := DefaultDirectives()
	utf8bom := []byte{0xEF, 0xBB, 0xBF}
	partialLine := ""

	parse := func(line string) error { // iterate line and accumulate children
		line, child, err := ParseLine(line, d)
		if err != nil {
			return err
		}
		partialLine = line

		if child != nil {
			partialLine = ""
			// Update the line information for the current child.
			child.startLine = startLine
			child.endLine = currentLine
			// Update the line information for the root. The starting line of the root is always the
			// starting line of the first child and the ending line is the ending line of the last child.
			if root.startLine < 0 {
				root.startLine = currentLine
			}
			root.endLine = currentLine
			root.Children = append(root.Children, child)
		}
		return nil
	}

	for scanner.Scan() {
		scannedBytes := scanner.Bytes()
		// We trim UTF8 BOM
		if currentLine == 0 {
			scannedBytes = bytes.TrimPrefix(scannedBytes, utf8bom)
		}
		currentLine++
		if parseDirectives {
			if err := ParseDirectives(string(scannedBytes), &d); err != nil {
				if err == errNoDirective {
					parseDirectives = false
				} else {
					return nil, err
				}
			}
		}

		scannedLine := string(scannedBytes)
		if len(partialLine) == 0 {
			startLine = currentLine
			scannedLine = strings.TrimLeftFunc(scannedLine, unicode.IsSpace)
		}
		if line := stripComments(scannedLine); line == "" {
			continue
		}
		partialLine += scannedLine
		if err := parse(partialLine); err != nil {
			return nil, err
		}
	}
	if partialLine != "" {
		if err := parse(partialLine); err != nil {
			return nil, err
		}
	}

	return root, nil
}
