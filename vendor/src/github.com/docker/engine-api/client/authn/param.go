package authn

import (
	"fmt"
	"strings"
)

// strspn counts the number of initial characters in s which are contained in
// spn, returning a value in the range of [0,len(s)].
func strspn(s, spn string) int {
	ret := strings.IndexFunc(s, func(r rune) bool { return strings.IndexRune(spn, r) == -1 })
	if ret == -1 {
		ret = len(s)
	}
	return ret
}

// tokenize splits an authentication challenge header value into a slice of
// strings, the first being the challenge itself, and each subsequent entry
// being a key/value pair, with quoting removed.  This isn't strictly in
// conformance with the RFCs, which allow multiple such sequences to be
// included in a single header line, but the Docker daemon doesn't do that, so
// we make the assumption.
func tokenize(header string) (challenge []string, err error) {
	token68 := "-._~+/0123456789" +
		"abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	var tokens []string
	var current int
	// build an intermediate list of unquoted items
	for current < len(header) {
		if len(tokens) > 0 && header[current] == '"' {
			// quoted string, can't be the first token
			quoted := ""
			end := current + 1
			for end < len(header) {
				if header[end] == '\\' {
					if end+1 < len(header) {
						end++
						quoted = quoted + string(header[end])
					}
					end++
					continue
				}
				if header[end] == '"' {
					end++
					break
				}
				quoted = quoted + string(header[end])
				end++
			}
			tokens = append(tokens, quoted)
			current = end
			continue
		}
		if strings.IndexAny(string(header[current]), token68) != -1 {
			// not a quoted string
			end := current + strspn(header[current:], token68)
			tokens = append(tokens, header[current:end])
			current = end
			continue
		}
		if header[current] == '=' {
			// treat "=" specifically
			tokens = append(tokens, string(header[current]))
			current++
			continue
		}
		if strings.IndexAny(string(header[current]), "\t ,\r\n") != -1 {
			// treat all whitespace as optional
			current++
			continue
		}
		return nil, fmt.Errorf("Error parsing header %v: unexpected token at %s", header, header[current:])
	}
	// enforce single token + optional key=value tuples as the format
	for i := 0; i < len(tokens); i++ {
		if (tokens[i] == "=") != (i%3 == 2) {
			return nil, fmt.Errorf("Error parsing token list %v: unexpected token %s", tokens, tokens[i])
		}
	}
	if len(tokens)%3 != 1 {
		return nil, fmt.Errorf("Error parsing token list %v: unexpected length", tokens)
	}
	// reformat as single token + key=value tokens
	challenge = append(challenge, tokens[0])
	for i := 1; i+2 < len(tokens); i += 3 {
		challenge = append(challenge, strings.ToLower(tokens[i])+"="+tokens[i+2])
	}
	return challenge, nil
}

// getParameter reads a particular parameter from an authentication challenge,
// returning an error if it runs into trouble parsing the challenge line
func getParameter(header string, parameter string) (string, error) {
	tokens, err := tokenize(header)
	if err != nil {
		return "", err
	}
	for _, token := range tokens {
		if strings.HasPrefix(token, strings.ToLower(parameter)+"=") {
			return token[len(parameter)+1:], nil
		}
	}
	return "", nil
}
