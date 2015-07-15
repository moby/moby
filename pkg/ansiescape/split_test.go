package ansiescape

import (
	"bufio"
	"strings"
	"testing"
)

func TestSplit(t *testing.T) {
	lines := []string{
		"test line 1",
		"another test line",
		"some test line",
		"line with non-cursor moving sequence \x1b[1T", // Scroll Down
		"line with \x1b[31;1mcolor\x1b[0m then reset",  // "color" in Bold Red
		"cursor forward \x1b[1C and backward \x1b[1D",
		"invalid sequence \x1babcd",
		"",
		"after empty",
	}
	splitSequences := []string{
		"\x1b[1A",   // Cursor up
		"\x1b[1B",   // Cursor down
		"\x1b[1E",   // Cursor next line
		"\x1b[1F",   // Cursor previous line
		"\x1b[1;1H", // Move cursor to position
		"\x1b[1;1h", // Move cursor to position
		"\n",
		"\r\n",
		"\n\r",
		"\x1b[1A\r",
		"\r\x1b[1A",
	}

	for _, sequence := range splitSequences {
		scanner := bufio.NewScanner(strings.NewReader(strings.Join(lines, sequence)))
		scanner.Split(ScanANSILines)
		i := 0
		for scanner.Scan() {
			if i >= len(lines) {
				t.Fatalf("Too many scanned lines")
			}
			scanned := scanner.Text()
			if scanned != lines[i] {
				t.Fatalf("Wrong line scanned with sequence %q\n\tExpected: %q\n\tActual:   %q", sequence, lines[i], scanned)
			}
			i++
		}
		if i < len(lines) {
			t.Errorf("Wrong number of lines for sequence %q: %d, expected %d", sequence, i, len(lines))
		}
	}
}
