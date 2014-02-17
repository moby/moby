package opts

import (
	"bufio"
	"bytes"
	"io"
	"os"
)

/*
Read in a line delimited file with environment variables enumerated
*/
func ParseEnvFile(filename string) ([]string, error) {
	fh, err := os.Open(filename)
	if err != nil {
		return []string{}, err
	}
	var (
		lines       []string = []string{}
		line, chunk []byte
	)
	reader := bufio.NewReader(fh)
	line, isPrefix, err := reader.ReadLine()

	for err == nil {
		if isPrefix {
			chunk = append(chunk, line...)
		} else if !isPrefix && len(chunk) > 0 {
			line = chunk
			chunk = []byte{}
		} else {
			chunk = []byte{}
		}

		if !isPrefix && len(line) > 0 && bytes.Contains(line, []byte("=")) {
			lines = append(lines, string(line))
		}
		line, isPrefix, err = reader.ReadLine()
	}
	if err != nil && err != io.EOF {
		return []string{}, err
	}
	return lines, nil
}
