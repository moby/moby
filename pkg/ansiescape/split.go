package ansiescape

import "bytes"

// dropCR drops a leading or terminal \r from the data.
func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		data = data[0 : len(data)-1]
	}
	if len(data) > 0 && data[0] == '\r' {
		data = data[1:]
	}
	return data
}

// escapeSequenceLength calculates the length of an ANSI escape sequence
// If there is not enough characters to match a sequence, -1 is returned,
// if there is no valid sequence 0 is returned, otherwise the number
// of bytes in the sequence is returned. Only returns length for
// line moving sequences.
func escapeSequenceLength(data []byte) int {
	next := 0
	if len(data) <= next {
		return -1
	}
	if data[next] != '[' {
		return 0
	}
	for {
		next = next + 1
		if len(data) <= next {
			return -1
		}
		if (data[next] > '9' || data[next] < '0') && data[next] != ';' {
			break
		}
	}
	if len(data) <= next {
		return -1
	}
	// Only match line moving codes
	switch data[next] {
	case 'A', 'B', 'E', 'F', 'H', 'h':
		return next + 1
	}

	return 0
}

// ScanANSILines is a scanner function which splits the
// input based on ANSI escape codes and new lines.
func ScanANSILines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Look for line moving escape sequence
	if i := bytes.IndexByte(data, '\x1b'); i >= 0 {
		last := 0
		for i >= 0 {
			last = last + i

			// get length of ANSI escape sequence
			sl := escapeSequenceLength(data[last+1:])
			if sl == -1 {
				return 0, nil, nil
			}
			if sl == 0 {
				// If no relevant sequence was found, skip
				last = last + 1
				i = bytes.IndexByte(data[last:], '\x1b')
				continue
			}

			return last + 1 + sl, dropCR(data[0:(last)]), nil
		}
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// No escape sequence, check for new line
		return i + 1, dropCR(data[0:i]), nil
	}

	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), dropCR(data), nil
	}
	// Request more data.
	return 0, nil, nil
}
