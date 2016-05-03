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

var (
	dispatch              map[string]func(string) (*Node, map[string]bool, error)
	tokenWhitespace       = regexp.MustCompile(`[\t\v\f\r ]+`)
	tokenLineContinuation *regexp.Regexp
	tokenEscape           rune
	tokenEscapeCommand    = regexp.MustCompile(`^#[ \t]*escape[ \t]*=[ \t]*(?P<escapechar>.).*$`)
	tokenComment          = regexp.MustCompile(`^#.*$`)
	lookingForDirectives  bool
	directiveEscapeSeen   bool
)

const defaultTokenEscape = "\\"

// setTokenEscape sets the default token for escaping characters in a Dockerfile.
func setTokenEscape(s string) error {
	if s != "`" && s != "\\" {
		return fmt.Errorf("invalid ESCAPE '%s'. Must be ` or \\", s)
	}
	tokenEscape = rune(s[0])
	tokenLineContinuation = regexp.MustCompile(`\` + s + `[ \t]*$`)
	return nil
}

func init() {
	// Dispatch Table. see line_parsers.go for the parse functions.
	// The command is parsed and mapped to the line parser. The line parser
	// receives the arguments but not the command, and returns an AST after
	// reformulating the arguments according to the rules in the parser
	// functions. Errors are propagated up by Parse() and the resulting AST can
	// be incorporated directly into the existing AST as a next.
	dispatch = map[string]func(string) (*Node, map[string]bool, error){
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

// ParseLine parse a line and return the remainder.
func ParseLine(line string) (string, *Node, error) {

	// Handle the parser directive '# escape=<char>. Parser directives must preceed
	// any builder instruction or other comments, and cannot be repeated.
	if lookingForDirectives {
		tecMatch := tokenEscapeCommand.FindStringSubmatch(strings.ToLower(line))
		if len(tecMatch) > 0 {
			if directiveEscapeSeen == true {
				return "", nil, fmt.Errorf("only one escape parser directive can be used")
			}
			for i, n := range tokenEscapeCommand.SubexpNames() {
				if n == "escapechar" {
					if err := setTokenEscape(tecMatch[i]); err != nil {
						return "", nil, err
					}
					directiveEscapeSeen = true
					return "", nil, nil
				}
			}
		}
	}

	lookingForDirectives = false

	if line = stripComments(line); line == "" {
		return "", nil, nil
	}

	if tokenLineContinuation.MatchString(line) {
		line = tokenLineContinuation.ReplaceAllString(line, "")
		return line, nil, nil
	}

	cmd, flags, args, err := splitCommand(line)
	if err != nil {
		return "", nil, err
	}

	node := &Node{}
	node.Value = cmd

	sexp, attrs, err := fullDispatch(cmd, args)
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
func Parse(rwc io.Reader) (*Node, error) {
	directiveEscapeSeen = false
	lookingForDirectives = true
	setTokenEscape(defaultTokenEscape) // Assume the default token for escape
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
		line, child, err := ParseLine(scannedLine)
		if err != nil {
			return nil, err
		}
		startLine := currentLine

		if line != "" && child == nil {
			for scanner.Scan() {
				newline := scanner.Text()
				currentLine++

				if stripComments(strings.TrimSpace(newline)) == "" {
					continue
				}

				line, child, err = ParseLine(line + newline)
				if err != nil {
					return nil, err
				}

				if child != nil {
					break
				}
			}
			if child == nil && line != "" {
				_, child, err = ParseLine(line)
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
