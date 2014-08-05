// This package implements a parser and parse tree dumper for Dockerfiles.
package parser

import (
	"bufio"
	"io"
	"regexp"
	"strings"
)

// Node is the building block of the AST this package will create.
//
// Nodes are structured to have a value, next, and child, the latter two of
// which are Nodes themselves.
//
// This terminology is unfortunately rather confusing, so here's a diagram.
// Anything after the ; is a comment.
//
//     (
//       (run "foo") ; value run, and next is a value foo.
//       (run "1" "2" "3") ;
//       (something (really cool))
//     )
//
// Will give you something like this:
//
//     &Node{
//       Value:"",
//       Child: &Node{Value: "run", Next: &Node{Value: "foo"}, Child: nil},
//       Next: &Node{Value:"", Child: &Node{Value:"run", Next: &Node{Value:`"1"`....
//
// ... and so on.
//
// The short and fast rule is that anything that starts with ( is a child of
// something. Anything which follows a previous statement is a next of
// something.
//
type Node struct {
	Value    string  // actual content
	Next     *Node   // the next item in the current sexp
	Children []*Node // the children of this sexp
}

var (
	dispatch                map[string]func(string) (*Node, error)
	TOKEN_WHITESPACE        = regexp.MustCompile(`[\t\v\f\r ]+`)
	TOKEN_LINE_CONTINUATION = regexp.MustCompile(`\\$`)
	TOKEN_COMMENT           = regexp.MustCompile(`^#.*$`)
)

func init() {
	// Dispatch Table. see line_parsers.go for the parse functions.
	// The command is parsed and mapped to the line parser. The line parser
	// recieves the arguments but not the command, and returns an AST after
	// reformulating the arguments according to the rules in the parser
	// functions. Errors are propogated up by Parse() and the resulting AST can
	// be incorporated directly into the existing AST as a next.
	dispatch = map[string]func(string) (*Node, error){
		"user":           parseString,
		"onbuild":        parseSubCommand,
		"workdir":        parseString,
		"env":            parseEnv,
		"maintainer":     parseString,
		"docker-version": parseString,
		"from":           parseString,
		"add":            parseStringsWhitespaceDelimited,
		"copy":           parseStringsWhitespaceDelimited,
		"run":            parseMaybeJSON,
		"cmd":            parseMaybeJSON,
		"entrypoint":     parseMaybeJSON,
		"expose":         parseStringsWhitespaceDelimited,
		"volume":         parseMaybeJSON,
		"insert":         parseIgnore,
	}
}

// empty node. Useful for managing structure.
func blankNode() *Node {
	return &Node{"", nil, []*Node{}}
}

func parseLine(line string) (string, *Node, error) {
	if line = stripComments(line); line == "" {
		return "", nil, nil
	}

	if TOKEN_LINE_CONTINUATION.MatchString(line) {
		line = TOKEN_LINE_CONTINUATION.ReplaceAllString(line, "")
		return line, nil, nil
	}

	cmd, args := splitCommand(line)

	node := blankNode()
	node.Value = cmd

	sexp, err := fullDispatch(cmd, args)
	if err != nil {
		return "", nil, err
	}

	node.Next = sexp

	return "", node, nil
}

// The main parse routine. Handles an io.ReadWriteCloser and returns the root
// of the AST.
func Parse(rwc io.Reader) (*Node, error) {
	var child *Node
	var line string
	var err error
	root := blankNode()
	scanner := bufio.NewScanner(rwc)

	for scanner.Scan() {
		line, child, err = parseLine(strings.TrimSpace(scanner.Text()))
		if err != nil {
			return nil, err
		}

		if line != "" && child == nil {
			for {
				scanner.Scan()
				newline := strings.TrimSpace(scanner.Text())

				if newline == "" {
					continue
				}

				line, child, err = parseLine(line + newline)
				if err != nil {
					return nil, err
				}

				if child != nil {
					break
				}
			}
		}

		if child != nil {
			root.Children = append(root.Children, child)
		}
	}

	return root, nil
}
