package opts

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

/*
Read in a line delimited file with environment variables enumerated
*/
func ParseEnvFile(filename string) ([]string, error) {
	fh, err := os.Open(filename)
	if err != nil {
		return []string{}, err
	}
	defer fh.Close()

	lines := []string{}
	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		line := scanner.Text()
		// line is not empty, and not starting with '#'
		if len(line) > 0 && !strings.HasPrefix(line, "#") {
			if strings.Contains(line, "=") {
				data := strings.SplitN(line, "=", 2)
				lines = append(lines, fmt.Sprintf("%s=%s", data[0], data[1]))
			} else {
				lines = append(lines, fmt.Sprintf("%s=%s", line, os.Getenv(line)))
			}
		}
	}
	return lines, nil
}
