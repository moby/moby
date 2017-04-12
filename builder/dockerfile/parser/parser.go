// Package parser implements a parser and parse tree dumper for Dockerfiles.
package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
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
	endLine    int             // the line in the original dockerfile where the node ends
}

// Dump dumps the AST defined by `node` as a list of sexps.
// Returns a string suitable for printing.
func (node *Node) Dump() string {
	str := ""
	str += node.Value

	if len(node.Flags) > 0 {
		str += fmt.Sprintf(" %q", node.Flags)
	}

	for _, n := range node.Children {
		str += "(" + n.Dump() + ")\n"
	}

	for n := node.Next; n != nil; n = n.Next {
		if len(n.Children) > 0 {
			str += " " + n.Dump()
		} else {
			str += " " + strconv.Quote(n.Value)
		}
	}

	return strings.TrimSpace(str)
}

var (
	dispatch           map[string]func(string, *Directive) (*Node, map[string]bool, error)
	tokenWhitespace    = regexp.MustCompile(`[\t\v\f\r ]+`)
	tokenEscapeCommand = regexp.MustCompile(`^#[ \t]*escape[ \t]*=[ \t]*(?P<escapechar>.).*$`)
	tokenComment       = regexp.MustCompile(`^#.*$`)
)

// DefaultEscapeToken is the default escape token
const DefaultEscapeToken = '\\'

// Directive is the structure used during a build run to hold the state of
// parsing directives.
type Directive struct {
	escapeToken           rune           // Current escape token
	lineContinuationRegex *regexp.Regexp // Current line continuation regex
	lookingForDirectives  bool           // Whether we are currently looking for directives
	escapeSeen            bool           // Whether the escape directive has been seen
}

// setEscapeToken sets the default token for escaping characters in a Dockerfile.
func (d *Directive) setEscapeToken(s string) error {
	if s != "`" && s != "\\" {
		return fmt.Errorf("invalid ESCAPE '%s'. Must be ` or \\", s)
	}
	d.escapeToken = rune(s[0])
	d.lineContinuationRegex = regexp.MustCompile(`\` + s + `[ \t]*$`)
	return nil
}

// NewDefaultDirective returns a new Directive with the default escapeToken token
func NewDefaultDirective() *Directive {
	directive := Directive{
		escapeSeen:           false,
		lookingForDirectives: true,
	}
	directive.setEscapeToken(string(DefaultEscapeToken))
	return &directive
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
		command.From:        parseStringsWhitespaceDelimited,
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
	if escapeFound, err := handleParserDirective(line, d); err != nil || escapeFound {
		d.escapeSeen = escapeFound
		return "", nil, err
	}

	d.lookingForDirectives = false

	if line = stripComments(line); line == "" {
		return "", nil, nil
	}

	if !ignoreCont && d.lineContinuationRegex.MatchString(line) {
		line = d.lineContinuationRegex.ReplaceAllString(line, "")
		return line, nil, nil
	}

	node, err := newNodeFromLine(line, d)
	return "", node, err
}

// newNodeFromLine splits the line into parts, and dispatches to a function
// based on the command and command arguments. A Node is created from the
// result of the dispatch.
func newNodeFromLine(line string, directive *Directive) (*Node, error) {
	cmd, flags, args, err := splitCommand(line)
	if err != nil {
		return nil, err
	}

	fn := dispatch[cmd]
	// Ignore invalid Dockerfile instructions
	if fn == nil {
		fn = parseIgnore
	}
	next, attrs, err := fn(args, directive)
	if err != nil {
		return nil, err
	}

	return &Node{
		Value:      cmd,
		Original:   line,
		Flags:      flags,
		Next:       next,
		Attributes: attrs,
	}, nil
}

// Handle the parser directive '# escapeToken=<char>. Parser directives must precede
// any builder instruction or other comments, and cannot be repeated.
func handleParserDirective(line string, d *Directive) (bool, error) {
	if !d.lookingForDirectives {
		return false, nil
	}
	tecMatch := tokenEscapeCommand.FindStringSubmatch(strings.ToLower(line))
	if len(tecMatch) == 0 {
		return false, nil
	}
	if d.escapeSeen == true {
		return false, fmt.Errorf("only one escape parser directive can be used")
	}
	for i, n := range tokenEscapeCommand.SubexpNames() {
		if n == "escapechar" {
			if err := d.setEscapeToken(tecMatch[i]); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

// Result is the result of parsing a Dockerfile
type Result struct {
	AST         *Node
	EscapeToken rune
}

// Parse reads lines from a Reader, parses the lines into an AST and returns
// the AST and escape token
func Parse(rwc io.Reader) (*Result, error) {
	d := NewDefaultDirective()
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

				if stripComments(strings.TrimSpace(newline)) == "" {
					continue
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
			child.endLine = currentLine
			// Update the line information for the root. The starting line of the root is always the
			// starting line of the first child and the ending line is the ending line of the last child.
			if root.StartLine < 0 {
				root.StartLine = currentLine
			}
			root.endLine = currentLine
			root.Children = append(root.Children, child)
		}
	}

	return &Result{AST: root, EscapeToken: d.escapeToken}, nil
}

// covers comments and empty lines. Lines should be trimmed before passing to
// this function.
func stripComments(line string) string {
	// string is already trimmed at this point
	if tokenComment.MatchString(line) {
		return tokenComment.ReplaceAllString(line, "")
	}

	return line
}
