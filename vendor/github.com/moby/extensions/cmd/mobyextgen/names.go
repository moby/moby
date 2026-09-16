package main

import "strings"

// helpers

// camelToSnake converts Go field names to proto3 snake_case while keeping
// initialisms together (ContainerID -> container_id, HTTPServer -> http_server).
// A trailing s remains part of an initialism's plural form.
func camelToSnake(s string) string {
	r := []rune(s)
	var b strings.Builder
	for i, c := range r {
		if i > 0 && c >= 'A' && c <= 'Z' {
			prev := r[i-1]
			prevIsLowerOrDigit := (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9')
			nextIsLower := i+1 < len(r) && r[i+1] >= 'a' && r[i+1] <= 'z'
			pluralS := nextIsLower && r[i+1] == 's' && (i+2 == len(r) || (r[i+2] >= 'A' && r[i+2] <= 'Z'))
			if prevIsLowerOrDigit || (prev >= 'A' && prev <= 'Z' && nextIsLower && !pluralS) {
				b.WriteByte('_')
			}
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b.WriteRune(c)
	}
	return b.String()
}

// goCamelCase matches protoc-gen-go's field-name conversion so conversions use
// the generated struct and getter names.
func goCamelCase(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '.' && i+1 < len(s) && isASCIILower(s[i+1]):
		case c == '.':
			b = append(b, '_')
		case c == '_' && (i == 0 || s[i-1] == '.'):
			b = append(b, 'X')
		case c == '_' && i+1 < len(s) && isASCIILower(s[i+1]):
		case isASCIIDigit(c):
			b = append(b, c)
		default:
			if isASCIILower(c) {
				c -= 'a' - 'A'
			}
			b = append(b, c)
			for ; i+1 < len(s) && isASCIILower(s[i+1]); i++ {
				b = append(b, s[i+1])
			}
		}
	}
	return string(b)
}

func isASCIILower(c byte) bool { return 'a' <= c && c <= 'z' }
func isASCIIDigit(c byte) bool { return '0' <= c && c <= '9' }

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// singleDoc and singleField carry DefineSinglePoint cardinality into generated
// registration.
func singleDoc(pt point) string {
	if !pt.isSingle {
		return ""
	}
	return "\n// Single carries the contract's cardinality: the point admits one provider."
}

func singleField(pt point) string {
	if !pt.isSingle {
		return ""
	}
	return ", Single: true"
}

func methodConst(m method) string { return "method" + m.name }
func handlerName(m method) string { return "handle" + m.name }
