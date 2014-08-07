package parser

// line parsers are dispatch calls that parse a single unit of text into a
// Node object which contains the whole statement. Dockerfiles have varied
// (but not usually unique, see ONBUILD for a unique example) parsing rules
// per-command, and these unify the processing in a way that makes it
// manageable.

import (
	"encoding/json"
	"strconv"
	"strings"
)

// ignore the current argument. This will still leave a command parsed, but
// will not incorporate the arguments into the ast.
func parseIgnore(rest string) (*Node, error) {
	return blankNode(), nil
}

// used for onbuild. Could potentially be used for anything that represents a
// statement with sub-statements.
//
// ONBUILD RUN foo bar -> (onbuild (run foo bar))
//
func parseSubCommand(rest string) (*Node, error) {
	_, child, err := parseLine(rest)
	if err != nil {
		return nil, err
	}

	return &Node{Children: []*Node{child}}, nil
}

// parse environment like statements. Note that this does *not* handle
// variable interpolation, which will be handled in the evaluator.
func parseEnv(rest string) (*Node, error) {
	node := blankNode()
	rootnode := node
	strs := TOKEN_WHITESPACE.Split(rest, 2)
	node.Value = strs[0]
	node.Next = blankNode()
	node.Next.Value = strs[1]

	return rootnode, nil
}

// parses a whitespace-delimited set of arguments. The result is effectively a
// linked list of string arguments.
func parseStringsWhitespaceDelimited(rest string) (*Node, error) {
	node := blankNode()
	rootnode := node
	prevnode := node
	for _, str := range TOKEN_WHITESPACE.Split(rest, -1) { // use regexp
		prevnode = node
		node.Value = str
		node.Next = blankNode()
		node = node.Next
	}

	// XXX to get around regexp.Split *always* providing an empty string at the
	// end due to how our loop is constructed, nil out the last node in the
	// chain.
	prevnode.Next = nil

	return rootnode, nil
}

// parsestring just wraps the string in quotes and returns a working node.
func parseString(rest string) (*Node, error) {
	return &Node{rest, nil, nil}, nil
}

// parseJSON converts JSON arrays to an AST.
func parseJSON(rest string) (*Node, error) {
	var (
		myJson   []interface{}
		next     = blankNode()
		orignext = next
		prevnode = next
	)

	if err := json.Unmarshal([]byte(rest), &myJson); err != nil {
		return nil, err
	}

	for _, str := range myJson {
		switch str.(type) {
		case float64:
			str = strconv.FormatFloat(str.(float64), 'G', -1, 64)
		}
		next.Value = str.(string)
		next.Next = blankNode()
		prevnode = next
		next = next.Next
	}

	prevnode.Next = nil

	return orignext, nil
}

// parseMaybeJSON determines if the argument appears to be a JSON array. If
// so, passes to parseJSON; if not, quotes the result and returns a single
// node.
func parseMaybeJSON(rest string) (*Node, error) {
	rest = strings.TrimSpace(rest)

	if strings.HasPrefix(rest, "[") {
		node, err := parseJSON(rest)
		if err == nil {
			return node, nil
		}
	}

	node := blankNode()
	node.Value = rest
	return node, nil
}
