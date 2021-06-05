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

	"github.com/moby/buildkit/frontend/dockerfile/command"
	"github.com/moby/buildkit/frontend/dockerfile/shell"
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
	Value       string          // actual content
	Next        *Node           // the next item in the current sexp
	Children    []*Node         // the children of this sexp
	Heredocs    []Heredoc       // extra heredoc content attachments
	Attributes  map[string]bool // special attributes for this node
	Original    string          // original line used before parsing
	Flags       []string        // only top Node should have this set
	StartLine   int             // the line in the original dockerfile where the node begins
	EndLine     int             // the line in the original dockerfile where the node ends
	PrevComment []string
}

// Location return the location of node in source code
func (node *Node) Location() []Range {
	return toRanges(node.StartLine, node.EndLine)
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
	node.EndLine = end
}

func (node *Node) canContainHeredoc() bool {
	if _, allowedDirective := heredocDirectives[node.Value]; !allowedDirective {
		return false
	}
	if _, isJSON := node.Attributes["json"]; isJSON {
		return false
	}

	return true
}

// AddChild adds a new child node, and updates line information
func (node *Node) AddChild(child *Node, startLine, endLine int) {
	child.lines(startLine, endLine)
	if node.StartLine < 0 {
		node.StartLine = startLine
	}
	node.EndLine = endLine
	node.Children = append(node.Children, child)
}

type Heredoc struct {
	Name           string
	FileDescriptor uint
	Expand         bool
	Chomp          bool
	Content        string
}

var (
	dispatch          map[string]func(string, *directives) (*Node, map[string]bool, error)
	heredocDirectives map[string]bool
	reWhitespace      = regexp.MustCompile(`[\t\v\f\r ]+`)
	reDirectives      = regexp.MustCompile(`^#\s*([a-zA-Z][a-zA-Z0-9]*)\s*=\s*(.+?)\s*$`)
	reComment         = regexp.MustCompile(`^#.*$`)
	reHeredoc         = regexp.MustCompile(`^(\d*)<<(-?)(['"]?)([a-zA-Z][a-zA-Z0-9]*)(['"]?)$`)
	reLeadingTabs     = regexp.MustCompile(`(?m)^\t+`)
)

// DefaultEscapeToken is the default escape token
const DefaultEscapeToken = '\\'

var validDirectives = map[string]struct{}{
	"escape": {},
	"syntax": {},
}

// directive is the structure used during a build run to hold the state of
// parsing directives.
type directives struct {
	escapeToken           rune                // Current escape token
	lineContinuationRegex *regexp.Regexp      // Current line continuation regex
	done                  bool                // Whether we are done looking for directives
	seen                  map[string]struct{} // Whether the escape directive has been seen
}

// setEscapeToken sets the default token for escaping characters and as line-
// continuation token in a Dockerfile. Only ` (backtick) and \ (backslash) are
// allowed as token.
func (d *directives) setEscapeToken(s string) error {
	if s != "`" && s != `\` {
		return errors.Errorf("invalid escape token '%s' does not match ` or \\", s)
	}
	d.escapeToken = rune(s[0])
	// The escape token is used both to escape characters in a line and as line
	// continuation token. If it's the last non-whitespace token, it is used as
	// line-continuation token, *unless* preceded by an escape-token.
	//
	// The second branch in the regular expression handles line-continuation
	// tokens on their own line, which don't have any character preceding them.
	//
	// Due to Go lacking negative look-ahead matching, this regular expression
	// does not currently handle a line-continuation token preceded by an *escaped*
	// escape-token ("foo \\\").
	d.lineContinuationRegex = regexp.MustCompile(`([^\` + s + `])\` + s + `[ \t]*$|^\` + s + `[ \t]*$`)
	return nil
}

// possibleParserDirective looks for parser directives, eg '# escapeToken=<char>'.
// Parser directives must precede any builder instruction or other comments,
// and cannot be repeated.
func (d *directives) possibleParserDirective(line string) error {
	if d.done {
		return nil
	}

	match := reDirectives.FindStringSubmatch(line)
	if len(match) == 0 {
		d.done = true
		return nil
	}

	k := strings.ToLower(match[1])
	_, ok := validDirectives[k]
	if !ok {
		d.done = true
		return nil
	}

	if _, ok := d.seen[k]; ok {
		return errors.Errorf("only one %s parser directive can be used", k)
	}
	d.seen[k] = struct{}{}

	if k == "escape" {
		return d.setEscapeToken(match[2])
	}

	return nil
}

// newDefaultDirectives returns a new directives structure with the default escapeToken token
func newDefaultDirectives() *directives {
	d := &directives{
		seen: map[string]struct{}{},
	}
	d.setEscapeToken(string(DefaultEscapeToken))
	return d
}

func init() {
	// Dispatch Table. see line_parsers.go for the parse functions.
	// The command is parsed and mapped to the line parser. The line parser
	// receives the arguments but not the command, and returns an AST after
	// reformulating the arguments according to the rules in the parser
	// functions. Errors are propagated up by Parse() and the resulting AST can
	// be incorporated directly into the existing AST as a next.
	dispatch = map[string]func(string, *directives) (*Node, map[string]bool, error){
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
func newNodeFromLine(line string, d *directives, comments []string) (*Node, error) {
	cmd, flags, args, err := splitCommand(line)
	if err != nil {
		return nil, err
	}

	fn := dispatch[cmd]
	// Ignore invalid Dockerfile instructions
	if fn == nil {
		fn = parseIgnore
	}
	next, attrs, err := fn(args, d)
	if err != nil {
		return nil, err
	}

	return &Node{
		Value:       cmd,
		Original:    line,
		Flags:       flags,
		Next:        next,
		Attributes:  attrs,
		PrevComment: comments,
	}, nil
}

// Result is the result of parsing a Dockerfile
type Result struct {
	AST         *Node
	EscapeToken rune
	Warnings    []string
}

// PrintWarnings to the writer
func (r *Result) PrintWarnings(out io.Writer) {
	if len(r.Warnings) == 0 {
		return
	}
	fmt.Fprintf(out, strings.Join(r.Warnings, "\n")+"\n")
}

// Parse reads lines from a Reader, parses the lines into an AST and returns
// the AST and escape token
func Parse(rwc io.Reader) (*Result, error) {
	d := newDefaultDirectives()
	currentLine := 0
	root := &Node{StartLine: -1}
	scanner := bufio.NewScanner(rwc)
	scanner.Split(scanLines)
	warnings := []string{}
	var comments []string

	var err error
	for scanner.Scan() {
		bytesRead := scanner.Bytes()
		if currentLine == 0 {
			// First line, strip the byte-order-marker if present
			bytesRead = bytes.TrimPrefix(bytesRead, utf8bom)
		}
		if isComment(bytesRead) {
			comment := strings.TrimSpace(string(bytesRead[1:]))
			if comment == "" {
				comments = nil
			} else {
				comments = append(comments, comment)
			}
		}
		bytesRead, err = processLine(d, bytesRead, true)
		if err != nil {
			return nil, withLocation(err, currentLine, 0)
		}
		currentLine++

		startLine := currentLine
		line, isEndOfLine := trimContinuationCharacter(string(bytesRead), d)
		if isEndOfLine && line == "" {
			continue
		}

		var hasEmptyContinuationLine bool
		for !isEndOfLine && scanner.Scan() {
			bytesRead, err := processLine(d, scanner.Bytes(), false)
			if err != nil {
				return nil, withLocation(err, currentLine, 0)
			}
			currentLine++

			if isComment(scanner.Bytes()) {
				// original line was a comment (processLine strips comments)
				continue
			}
			if isEmptyContinuationLine(bytesRead) {
				hasEmptyContinuationLine = true
				continue
			}

			continuationLine := string(bytesRead)
			continuationLine, isEndOfLine = trimContinuationCharacter(continuationLine, d)
			line += continuationLine
		}

		if hasEmptyContinuationLine {
			warnings = append(warnings, "[WARNING]: Empty continuation line found in:\n    "+line)
		}

		child, err := newNodeFromLine(line, d, comments)
		if err != nil {
			return nil, withLocation(err, startLine, currentLine)
		}

		if child.canContainHeredoc() {
			heredocs, err := heredocsFromLine(line)
			if err != nil {
				return nil, withLocation(err, startLine, currentLine)
			}

			for _, heredoc := range heredocs {
				terminator := []byte(heredoc.Name)
				terminated := false
				for scanner.Scan() {
					bytesRead := scanner.Bytes()
					currentLine++

					possibleTerminator := trimNewline(bytesRead)
					if heredoc.Chomp {
						possibleTerminator = trimLeadingTabs(possibleTerminator)
					}
					if bytes.Equal(possibleTerminator, terminator) {
						terminated = true
						break
					}
					heredoc.Content += string(bytesRead)
				}
				if !terminated {
					return nil, withLocation(errors.New("unterminated heredoc"), startLine, currentLine)
				}

				child.Heredocs = append(child.Heredocs, heredoc)
			}
		}

		root.AddChild(child, startLine, currentLine)
		comments = nil
	}

	if len(warnings) > 0 {
		warnings = append(warnings, "[WARNING]: Empty continuation lines will become errors in a future release.")
	}

	if root.StartLine < 0 {
		return nil, withLocation(errors.New("file with no instructions"), currentLine, 0)
	}

	return &Result{
		AST:         root,
		Warnings:    warnings,
		EscapeToken: d.escapeToken,
	}, withLocation(handleScannerError(scanner.Err()), currentLine, 0)
}

func heredocFromMatch(match []string) (*Heredoc, error) {
	if len(match) == 0 {
		return nil, nil
	}

	fileDescriptor, _ := strconv.ParseUint(match[1], 10, 0)
	chomp := match[2] == "-"
	quoteOpen := match[3]
	name := match[4]
	quoteClose := match[5]

	expand := true
	if quoteOpen != "" || quoteClose != "" {
		if quoteOpen != quoteClose {
			return nil, errors.New("quoted heredoc quotes do not match")
		}
		expand = false
	}

	return &Heredoc{
		Name:           name,
		Expand:         expand,
		Chomp:          chomp,
		FileDescriptor: uint(fileDescriptor),
	}, nil
}

func ParseHeredoc(src string) (*Heredoc, error) {
	return heredocFromMatch(reHeredoc.FindStringSubmatch(src))
}
func MustParseHeredoc(src string) *Heredoc {
	heredoc, _ := ParseHeredoc(src)
	return heredoc
}

func heredocsFromLine(line string) ([]Heredoc, error) {
	shlex := shell.NewLex('\\')
	shlex.RawQuotes = true
	words, _ := shlex.ProcessWords(line, []string{})

	var docs []Heredoc
	for _, word := range words {
		heredoc, err := ParseHeredoc(word)
		if err != nil {
			return nil, err
		}
		if heredoc != nil {
			docs = append(docs, *heredoc)
		}
	}
	return docs, nil
}

func ChompHeredocContent(src string) string {
	return reLeadingTabs.ReplaceAllString(src, "")
}

func trimComments(src []byte) []byte {
	return reComment.ReplaceAll(src, []byte{})
}

func trimLeadingWhitespace(src []byte) []byte {
	return bytes.TrimLeftFunc(src, unicode.IsSpace)
}
func trimLeadingTabs(src []byte) []byte {
	return bytes.TrimLeft(src, "\t")
}
func trimNewline(src []byte) []byte {
	return bytes.TrimRight(src, "\r\n")
}

func isComment(line []byte) bool {
	return reComment.Match(trimLeadingWhitespace(trimNewline(line)))
}

func isEmptyContinuationLine(line []byte) bool {
	return len(trimLeadingWhitespace(trimNewline(line))) == 0
}

var utf8bom = []byte{0xEF, 0xBB, 0xBF}

func trimContinuationCharacter(line string, d *directives) (string, bool) {
	if d.lineContinuationRegex.MatchString(line) {
		line = d.lineContinuationRegex.ReplaceAllString(line, "$1")
		return line, false
	}
	return line, true
}

// TODO: remove stripLeftWhitespace after deprecation period. It seems silly
// to preserve whitespace on continuation lines. Why is that done?
func processLine(d *directives, token []byte, stripLeftWhitespace bool) ([]byte, error) {
	token = trimNewline(token)
	if stripLeftWhitespace {
		token = trimLeadingWhitespace(token)
	}
	return trimComments(token), d.possibleParserDirective(string(token))
}

// Variation of bufio.ScanLines that preserves the line endings
func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return i + 1, data[0 : i+1], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func handleScannerError(err error) error {
	switch err {
	case bufio.ErrTooLong:
		return errors.Errorf("dockerfile line greater than max allowed size of %d", bufio.MaxScanTokenSize-1)
	default:
		return err
	}
}
