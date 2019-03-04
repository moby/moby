package opts

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

var whiteSpaces = " \t"

// ErrBadKey typed error for bad environment variable
type ErrBadKey struct {
	msg string
}

func (e ErrBadKey) Error() string {
	return fmt.Sprintf("poorly formatted environment: %s", e.msg)
}

func parseKeyValueFile(filename string, emptyFn func(string) (string, bool)) ([]string, error) {
	fh, err := os.Open(filename)
	if err != nil {
		return []string{}, err
	}
	defer fh.Close()

	lines := []string{}
	scanner := bufio.NewScanner(fh)
	currentLine := 0
	utf8bom := []byte{0xEF, 0xBB, 0xBF}
	for scanner.Scan() {
		scannedBytes := scanner.Bytes()
		if !utf8.Valid(scannedBytes) {
			return []string{}, fmt.Errorf("env file %s contains invalid utf8 bytes at line %d: %v", filename, currentLine+1, scannedBytes)
		}
		// We trim UTF8 BOM
		if currentLine == 0 {
			scannedBytes = bytes.TrimPrefix(scannedBytes, utf8bom)
		}
		// trim the line from all leading whitespace first
		line := strings.TrimLeftFunc(string(scannedBytes), unicode.IsSpace)
		currentLine++
		// line is not empty, and not starting with '#'
		if len(line) > 0 && !strings.HasPrefix(line, "#") {
			data := strings.SplitN(line, "=", 2)

			// trim the front of a variable, but nothing else
			variable := strings.TrimLeft(data[0], whiteSpaces)
			if strings.ContainsAny(variable, whiteSpaces) {
				return []string{}, ErrBadKey{fmt.Sprintf("variable '%s' has white spaces", variable)}
			}
			if len(variable) == 0 {
				return []string{}, ErrBadKey{fmt.Sprintf("no variable name on line '%s'", line)}
			}

			if len(data) > 1 {
				// pass the value through, no trimming
				lines = append(lines, fmt.Sprintf("%s=%s", variable, data[1]))
			} else {
				var value string
				var present bool
				if emptyFn != nil {
					value, present = emptyFn(line)
				}
				if present {
					// if only a pass-through variable is given, clean it up.
					lines = append(lines, fmt.Sprintf("%s=%s", strings.TrimSpace(line), value))
				}
			}
		}
	}
	return lines, scanner.Err()
}
