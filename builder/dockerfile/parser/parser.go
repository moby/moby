// Package parser implements a parser and parse tree dumper for Dockerfiles.
package parser

import (
	"bufio"
	"bytes"
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
	StartLine  int             // the line in the original dockerfile where the node begins
	EndLine    int             // the line in the original dockerfile where the node ends
}

// Directive is the structure used during a build run to hold the state of
// parsing directives.
type Directive struct {
	EscapeToken           rune           // Current escape token
	LineContinuationRegex *regexp.Regexp // Current line contination regex
	LookingForDirectives  bool           // Whether we are currently looking for directives
	EscapeSeen            bool           // Whether the escape directive has been seen
}

var (
	dispatch           map[string]func(string, *Directive) (*Node, map[string]bool, error)
	tokenWhitespace    = regexp.MustCompile(`[\t\v\f\r ]+`)
	tokenEscapeCommand = regexp.MustCompile(`^#[ \t]*escape[ \t]*=[ \t]*(?P<escapechar>.).*$`)
	tokenComment       = regexp.MustCompile(`^#.*$`)
)

// DefaultEscapeToken is the default escape token
const DefaultEscapeToken = "\\"

// SetEscapeToken sets the default token for escaping characters in a Dockerfile.
func SetEscapeToken(s string, d *Directive) error {
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
	dispatch = map[string]func(string, *Directive) (*Node, map[string]bool, error){
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
func ParseLine(line string, d *Directive, ignoreCont bool) (string, *Node, error) {
	// Handle the parser directive '# escape=<char>. Parser directives must precede
	// any builder instruction or other comments, and cannot be repeated.
	if d.LookingForDirectives {
		tecMatch := tokenEscapeCommand.FindStringSubmatch(strings.ToLower(line))
		if len(tecMatch) > 0 {
			if d.EscapeSeen == true {
				return "", nil, fmt.Errorf("only one escape parser directive can be used")
			}
			for i, n := range tokenEscapeCommand.SubexpNames() {
				if n == "escapechar" {
					if err := SetEscapeToken(tecMatch[i], d); err != nil {
						return "", nil, err
					}
					d.EscapeSeen = true
					return "", nil, nil
				}
			}
		}
	}

	d.LookingForDirectives = false

	if line = stripComments(line); line == "" {
		return "", nil, nil
	}

	if !ignoreCont && d.LineContinuationRegex.MatchString(line) {
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

// Parse is the main parse routine.
// It handles an io.ReadWriteCloser and returns the root of the AST.
func Parse(rwc io.Reader, d *Directive) (*Node, error) {
	currentLine := 0
	root := &Node{}
	root.StartLine = -1
	scanner := bufio.NewScanner(rwc)

	utf8bom := []byte{0xEF, 0xBB, 0xBF}
	for scanner.Scan() {
		scannedBytes := scanner.Bytes()
		// We trim UTF8 BOM
		if currentLine == 0 {
			scannedBytes = bytes.TrimPrefix(scannedBytes, utf8bom)
		}
		scannedLine := strings.TrimLeftFunc(string(scannedBytes), unicode.IsSpace)
		currentLine++
		line, child, err := ParseLine(scannedLine, d, false)
		if err != nil {
			return nil, err
		}
		startLine := currentLine

		if line != "" && child == nil {
			for scanner.Scan() {
				newline := scanner.Text()
				currentLine++

				// If escape followed by a comment line then stop
				// Note here that comment line starts with `#` at
				// the first pos of the line
				if stripComments(newline) == "" {
					break
				}

				// If escape followed by an empty line then stop
				if strings.TrimSpace(newline) == "" {
					break
				}
				line, child, err = ParseLine(line+newline, d, false)
				if err != nil {
					return nil, err
				}

				if child != nil {
					break
				}
			}
			if child == nil && line != "" {
				// When we call ParseLine we'll pass in 'true' for
				// the ignoreCont param if we're at the EOF. This will
				// prevent the func from returning immediately w/o
				// parsing the line thinking that there's more input
				// to come.

				_, child, err = ParseLine(line, d, scanner.Err() == nil)
				if err != nil {
					return nil, err
				}
			}
		}

		if child != nil {
			// Update the line information for the current child.
			child.StartLine = startLine
			child.EndLine = currentLine
			// Update the line information for the root. The starting line of the root is always the
			// starting line of the first child and the ending line is the ending line of the last child.
			if root.StartLine < 0 {
				root.StartLine = currentLine
			}
			root.EndLine = currentLine
			root.Children = append(root.Children, child)
		}
	}

	return root, nil
}
