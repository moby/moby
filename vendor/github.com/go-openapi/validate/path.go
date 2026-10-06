// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"
	"strconv"
	"strings"

	"github.com/go-openapi/jsonpointer"
)

// pathSegments is the location of a validated value inside a document,
// held as an ordered list of unescaped JSON pointer reference tokens.
//
// Validators build a location by appending tokens as they descend into
// properties and array items, then render it only when they report an error.
// Keeping the tokens apart until then makes it possible to produce a
// valid [RFC 6901] JSON pointer: a token is escaped when it is rendered, and
// the separator can never be confused with a token that contains one.
//
// The zero value is the location of the document root.
//
// [RFC 6901]: https://datatracker.ietf.org/doc/html/rfc6901
type pathSegments []pathToken

// pathToken is one step of a location.
//
// A token addresses a member the way the document does, which is not always
// the way a reader recognizes it: an operation addresses its parameters by
// index, while a message is far more useful naming them. When the two differ,
// display carries the readable form and only the message uses it.
type pathToken struct {
	token   string
	display string

	// structural marks a token a document needs to address the value, but
	// that messages have never shown: a "properties" between a schema and one
	// of its members, say. It is part of the pointer and absent from the
	// dotted form.
	structural bool

	// cosmetic is the converse: a token messages name but that the document
	// does not address, such as a parameter too broken to be found by name in
	// the array holding it. It is part of the dotted form and absent from the
	// pointer, which then stops at the deepest node the document does contain.
	cosmetic bool
}

// readable renders a token the way a message should spell it.
func (t pathToken) readable() string {
	if t.display != "" {
		return t.display
	}

	return t.token
}

// newPathSegments builds a location from a list of unescaped tokens.
func newPathSegments(tokens ...string) pathSegments {
	if len(tokens) == 0 {
		return nil
	}

	segments := make(pathSegments, len(tokens))
	for i, token := range tokens {
		segments[i] = pathToken{token: token}
	}

	return segments
}

// rootPath is the location of the document root.
func rootPath() pathSegments { return nil }

// String implements [fmt.Stringer] with the legacy dotted notation, so that a
// location interpolated into a message reads as it always has.
func (p pathSegments) String() string { return p.dotted() }

// child returns the location of a named member of the value at p.
//
// The receiver is never modified: sibling children may be derived from the
// same parent without aliasing one another.
func (p pathSegments) child(token string) pathSegments {
	return p.appendToken(pathToken{token: token})
}

// childAs returns the location of a member the document addresses as token,
// which messages should spell as display instead.
func (p pathSegments) childAs(token, display string) pathSegments {
	return p.appendToken(pathToken{token: token, display: display})
}

// structuralChild returns the location of a member a document addresses but
// messages do not name.
func (p pathSegments) structuralChild(token string) pathSegments {
	return p.appendToken(pathToken{token: token, structural: true})
}

// cosmeticChild returns a location that messages spell as a member named token,
// while the pointer stays on p.
func (p pathSegments) cosmeticChild(token string) pathSegments {
	return p.appendToken(pathToken{token: token, cosmetic: true})
}

func (p pathSegments) appendToken(token pathToken) pathSegments {
	child := make(pathSegments, len(p)+1)
	copy(child, p)
	child[len(p)] = p.inherit(token)

	return child
}

// inherit passes down what a parent token says about addressability: nothing
// below a token the document does not address is addressable either, so the
// pointer has to stop at the same place.
func (p pathSegments) inherit(token pathToken) pathToken {
	if len(p) > 0 && p[len(p)-1].cosmetic {
		token.cosmetic = true
	}

	return token
}

// children returns the location of a chain of named members below p.
func (p pathSegments) children(tokens ...string) pathSegments {
	child := make(pathSegments, len(p)+len(tokens))
	copy(child, p)
	for i, token := range tokens {
		child[len(p)+i] = child[:len(p)+i].inherit(pathToken{token: token})
	}

	return child
}

// item returns the location of the index'th element of the array at p.
func (p pathSegments) item(index int) pathSegments {
	return p.child(strconv.Itoa(index))
}

// isEmpty tells if p locates the document root.
func (p pathSegments) isEmpty() bool { return len(p) == 0 }

// last returns the trailing meaningful token, or an empty string at the
// document root.
//
// Structural tokens are skipped: they say how a document addresses the value,
// not what the value is, and the callers here are asking the latter.
func (p pathSegments) last() string {
	if token, ok := p.meaningfulAt(0); ok {
		return token
	}

	return ""
}

// beforeLast returns the meaningful token before the trailing one, or an empty
// string when p holds fewer than two of them.
func (p pathSegments) beforeLast() string {
	if token, ok := p.meaningfulAt(1); ok {
		return token
	}

	return ""
}

// meaningfulAt returns the nth token from the end, counting only the tokens a
// message would show.
func (p pathSegments) meaningfulAt(n int) (string, bool) {
	seen := 0
	for _, token := range slices.Backward(p) {
		if token.structural {
			continue
		}

		if seen == n {
			return token.token, true
		}

		seen++
	}

	return "", false
}

// trimIndexes returns p without its trailing array index tokens.
//
// It answers "what is this value inside of", disregarding how deep into an
// array it sits: the items of an example are still an example.
func (p pathSegments) trimIndexes() pathSegments {
	end := len(p)
	for end > 0 && isIndexToken(p[end-1].token) {
		end--
	}

	return p[:end]
}

// isIndexToken tells if a token addresses an array element rather than a member.
func isIndexToken(token string) bool {
	if token == "" {
		return false
	}

	for _, r := range token {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// hasSuffix tells if p ends with the given sequence of tokens.
func (p pathSegments) hasSuffix(suffix pathSegments) bool {
	if len(suffix) > len(p) {
		return false
	}

	offset := len(p) - len(suffix)
	for i, token := range suffix {
		if p[offset+i].token != token.token {
			return false
		}
	}

	return true
}

// dotted renders the location in the legacy dot-separated notation, e.g.
// "definitions.Pet.friends.0.name".
//
// Tokens are emitted verbatim: a token containing a dot is indistinguishable
// from a separator. This notation is kept because it is what surfaces as the
// name of a validation error, and API consumers of go-swagger servers see it.
// Use [pathSegments.pointer] whenever the location needs to be unambiguous.
func (p pathSegments) dotted() string {
	readable := make([]string, 0, len(p))
	for _, token := range p {
		if token.structural {
			continue
		}

		readable = append(readable, token.readable())
	}

	return strings.Join(readable, ".")
}

// pointer renders the location as an RFC 6901 JSON pointer, e.g.
// "/definitions/Pet/friends/0/name". The document root renders as "".
func (p pathSegments) pointer() string {
	if len(p) == 0 {
		return ""
	}

	var w strings.Builder
	for _, token := range p {
		if token.cosmetic {
			continue
		}

		w.WriteByte('/')
		w.WriteString(jsonpointer.Escape(token.token))
	}

	return w.String()
}
