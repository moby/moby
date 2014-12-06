package filters

import (
	"bufio"
	"bytes"
	"unicode/utf8"
)

func SplitByOperators(str string) ([]string, error) {
	argReader := bytes.NewBufferString(str)
	s := bufio.NewScanner(argReader)
	s.Split(ScanOperators)

	items := []string{}
	for s.Scan() {
		items = append(items, s.Text())
	}
	return items, nil
}

const Operators = "<>=!"

// a scanner splitter to get words and operators
func ScanOperators(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	i := bytes.IndexAny(data, Operators)
	if i == -1 {
		return bufio.ScanWords(data, atEOF)
	}

	// ensure the beginning is not a space or newline
	start := 0
	for width := 0; start < len(data); start += width {
		var r rune
		r, width = utf8.DecodeRune(data[start:])
		if !isSpace(r) {
			break
		}
	}

	if i != 0 {
		return i, data[start:i], nil
	}
	for width, j := 0, 0; j < len(data); j += width {
		var r rune
		r, width = utf8.DecodeRune(data[j:])
		if !isOperator(r) {
			return j, data[0:j], nil
		}
	}
	// Request more data.
	return 0, nil, nil

}
func isSpace(r rune) bool {
	return r == '\n' || r == '\r' || r == ' '
}
func isOperator(r rune) bool {
	for _, c := range Operators {
		if r == c {
			return true
		}
	}
	return false
}
