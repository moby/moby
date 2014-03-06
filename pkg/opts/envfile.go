package opts

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
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
		if len(line) > 0 && !strings.HasPrefix(line, "#") && strings.Contains(line, "=") {
			data := strings.SplitN(line, "=", 2)
			key := data[0]
			val := data[1]
			if str, err := strconv.Unquote(data[1]); err == nil {
				val = str
			}
			lines = append(lines, fmt.Sprintf("%s=%s", key, val))
		}
	}
	return lines, nil
}
