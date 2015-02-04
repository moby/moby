package parser

import (
	"strconv"
	"strings"
)

// dumps the AST defined by `node` as a list of sexps. Returns a string
// suitable for printing.
func (node *Node) Dump() string {
	str := ""
	str += node.Value

	for _, n := range node.Children {
		str += "(" + n.Dump() + ")\n"
	}

	if node.Next != nil {
		for n := node.Next; n != nil; n = n.Next {
			if len(n.Children) > 0 {
				str += " " + n.Dump()
			} else {
				str += " " + strconv.Quote(n.Value)
			}
		}
	}

	return strings.TrimSpace(str)
}

// performs the dispatch based on the two primal strings, cmd and args. Please
// look at the dispatch table in parser.go to see how these dispatchers work.
func fullDispatch(cmd, args string) (*Node, map[string]bool, error) {
	fn := dispatch[cmd]

	// Ignore invalid Dockerfile instructions
	if fn == nil {
		fn = parseIgnore
	}

	sexp, attrs, err := fn(args)
	if err != nil {
		return nil, nil, err
	}

	return sexp, attrs, nil
}

// splitCommand takes a single line of text and parses out the cmd and args,
// which are used for dispatching to more exact parsing functions.
func splitCommand(line string) (string, string, error) {
	var args string

	// Make sure we get the same results irrespective of leading/trailing spaces
	cmdline := TOKEN_WHITESPACE.Split(strings.TrimSpace(line), 2)
	cmd := strings.ToLower(cmdline[0])

	if len(cmdline) == 2 {
		args = strings.TrimSpace(cmdline[1])
	}

	// the cmd should never have whitespace, but it's possible for the args to
	// have trailing whitespace.
	return cmd, args, nil
}

// covers comments and empty lines. Lines should be trimmed before passing to
// this function.
func stripComments(line string) string {
	// string is already trimmed at this point
	if TOKEN_COMMENT.MatchString(line) {
		return TOKEN_COMMENT.ReplaceAllString(line, "")
	}

	return line
}
