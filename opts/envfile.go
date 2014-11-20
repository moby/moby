package opts

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/docker/docker/utils"
)

/*
Read in a line delimited file with environment variables enumerated
*/
func ParseEnvFile(filename string) ([]string, error) {
	var reader io.ReadCloser
	var err error
	if utils.IsURL(filename) {
		var resp *http.Response
		resp, err = utils.Download(filename)
		reader = resp.Body
	} else {
		reader, err = os.Open(filename)
	}
	if err != nil {
		return []string{}, err
	}
	defer reader.Close()
	return parseEnvFileFromReader(reader)
}

func parseEnvFileFromReader(reader io.Reader) ([]string, error) {
	lines := []string{}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		// line is not empty, and not starting with '#'
		if len(line) > 0 && !strings.HasPrefix(line, "#") {
			if strings.Contains(line, "=") {
				data := strings.SplitN(line, "=", 2)

				// trim the front of a variable, but nothing else
				variable := strings.TrimLeft(data[0], whiteSpaces)
				if strings.ContainsAny(variable, whiteSpaces) {
					return []string{}, ErrBadEnvVariable{fmt.Sprintf("variable '%s' has white spaces", variable)}
				}

				// pass the value through, no trimming
				lines = append(lines, fmt.Sprintf("%s=%s", variable, data[1]))
			} else {
				// if only a pass-through variable is given, clean it up.
				lines = append(lines, fmt.Sprintf("%s=%s", strings.TrimSpace(line), os.Getenv(line)))
			}
		}
	}
	return lines, nil
}

var whiteSpaces = " \t"

type ErrBadEnvVariable struct {
	msg string
}

func (e ErrBadEnvVariable) Error() string {
	return fmt.Sprintf("poorly formatted environment: %s", e.msg)
}
