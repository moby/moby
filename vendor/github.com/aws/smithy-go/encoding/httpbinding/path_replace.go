package httpbinding

import (
	"bytes"
	"fmt"
)

const (
	uriTokenStart = '{'
	uriTokenStop  = '}'
	uriTokenSkip  = '+'
)

func bufCap(b []byte, n int) []byte {
	if cap(b) < n {
		return make([]byte, 0, n)
	}

	return b[0:0]
}

// replacePathElement replaces a single element in the path []byte.
// Escape is used to control whether the value will be escaped using Amazon path escape style.
func replacePathElement(path, fieldBuf []byte, key, val string, escape bool) ([]byte, []byte, error) {
	fieldBuf = bufCap(fieldBuf, len(key)+3) // { <key> [+] }
	fieldBuf = append(fieldBuf, uriTokenStart)
	fieldBuf = append(fieldBuf, key...)

	start := bytes.Index(path, fieldBuf)
	end := start + len(fieldBuf)
	if start < 0 || len(path[end:]) == 0 {
		// TODO what to do about error?
		return path, fieldBuf, fmt.Errorf("invalid path index, start=%d,end=%d. %s", start, end, path)
	}

	encodeSep := true
	if path[end] == uriTokenSkip {
		// '+' token means do not escape slashes
		encodeSep = false
		end++
	}

	if escape {
		val = EscapePath(val, encodeSep)
	}

	if path[end] != uriTokenStop {
		return path, fieldBuf, fmt.Errorf("invalid path element, does not contain token stop, %s", path)
	}
	end++

	fieldBuf = bufCap(fieldBuf, len(val))
	fieldBuf = append(fieldBuf, val...)

	keyLen := end - start
	valLen := len(fieldBuf)

	if keyLen == valLen {
		copy(path[start:], fieldBuf)
		return path, fieldBuf, nil
	}

	newLen := len(path) + (valLen - keyLen)
	if len(path) < newLen {
		path = path[:cap(path)]
	}
	if cap(path) < newLen {
		newURI := make([]byte, newLen)
		copy(newURI, path)
		path = newURI
	}

	// shift
	copy(path[start+valLen:], path[end:])
	path = path[:newLen]
	copy(path[start:], fieldBuf)

	return path, fieldBuf, nil
}

// EscapePath escapes part of a URL path in Amazon style.
func EscapePath(path string, encodeSep bool) string {
	var buf bytes.Buffer
	for i := 0; i < len(path); i++ {
		c := path[i]
		if noEscape[c] || (c == '/' && !encodeSep) {
			buf.WriteByte(c)
		} else {
			fmt.Fprintf(&buf, "%%%02X", c)
		}
	}
	return buf.String()
}

var noEscape [256]bool

func init() {
	for i := 0; i < len(noEscape); i++ {
		// AWS expects every character except these to be escaped
		noEscape[i] = (i >= 'A' && i <= 'Z') ||
			(i >= 'a' && i <= 'z') ||
			(i >= '0' && i <= '9') ||
			i == '-' ||
			i == '.' ||
			i == '_' ||
			i == '~'
	}
}
