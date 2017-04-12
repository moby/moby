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
	"github.com/pkg/errors"
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

func (node *Node) lines(start, end int) {
	node.StartLine = start
	node.endLine = end
}

// AddChild adds a new child node, and updates line information
func (node *Node) AddChild(child *Node, startLine, endLine int) {
	child.lines(startLine, endLine)
	if node.StartLine < 0 {
		node.StartLine = startLine
	}
	node.endLine = endLine
	node.Children = append(node.Children, child)
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
	processingComplete    bool           // Whether we are done looking for directives
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

// processLine looks for a parser directive '# escapeToken=<char>. Parser
// directives must precede any builder instruction or other comments, and cannot
// be repeated.
func (d *Directive) processLine(line string) error {
	if d.processingComplete {
		return nil
	}
	// Processing is finished after the first call
	defer func() { d.processingComplete = true }()

	tecMatch := tokenEscapeCommand.FindStringSubmatch(strings.ToLower(line))
	if len(tecMatch) == 0 {
		return nil
	}
	if d.escapeSeen == true {
		return errors.New("only one escape parser directive can be used")
	}
	for i, n := range tokenEscapeCommand.SubexpNames() {
		if n == "escapechar" {
			d.escapeSeen = true
			return d.setEscapeToken(tecMatch[i])
		}
	}
	return nil
}

// NewDefaultDirective returns a new Directive with the default escapeToken token
func NewDefaultDirective() *Directive {
	directive := Directive{}
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
	root := &Node{StartLine: -1}
	scanner := bufio.NewScanner(rwc)

	var err error
	for scanner.Scan() {
		bytes := scanner.Bytes()
		switch currentLine {
		case 0:
			bytes, err = processFirstLine(d, bytes)
			if err != nil {
				return nil, err
			}
		default:
			bytes = processLine(bytes, true)
		}
		currentLine++

		startLine := currentLine
		line, isEndOfLine := trimContinuationCharacter(string(bytes), d)
		if isEndOfLine && line == "" {
			continue
		}

		for !isEndOfLine && scanner.Scan() {
			bytes := processLine(scanner.Bytes(), false)
			currentLine++

			// TODO: warn this is being deprecated/removed
			if isEmptyContinuationLine(bytes) {
				continue
			}

			continuationLine := string(bytes)
			continuationLine, isEndOfLine = trimContinuationCharacter(continuationLine, d)
			line += continuationLine
		}

		child, err := newNodeFromLine(line, d)
		if err != nil {
			return nil, err
		}
		root.AddChild(child, startLine, currentLine)
	}

	return &Result{AST: root, EscapeToken: d.escapeToken}, nil
}

func trimComments(src []byte) []byte {
	return tokenComment.ReplaceAll(src, []byte{})
}

func trimWhitespace(src []byte) []byte {
	return bytes.TrimLeftFunc(src, unicode.IsSpace)
}

func isEmptyContinuationLine(line []byte) bool {
	return len(trimComments(trimWhitespace(line))) == 0
}

var utf8bom = []byte{0xEF, 0xBB, 0xBF}

func trimContinuationCharacter(line string, d *Directive) (string, bool) {
	if d.lineContinuationRegex.MatchString(line) {
		line = d.lineContinuationRegex.ReplaceAllString(line, "")
		return line, false
	}
	return line, true
}

// TODO: remove stripLeftWhitespace after deprecation period. It seems silly
// to preserve whitespace on continuation lines. Why is that done?
func processLine(token []byte, stripLeftWhitespace bool) []byte {
	if stripLeftWhitespace {
		token = trimWhitespace(token)
	}
	return trimComments(token)
}

func processFirstLine(d *Directive, token []byte) ([]byte, error) {
	token = bytes.TrimPrefix(token, utf8bom)
	token = trimWhitespace(token)
	err := d.processLine(string(token))
	return trimComments(token), err
}
