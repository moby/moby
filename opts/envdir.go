package opts

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// ParseEnvDir Read in a directory with files acting as environment variables.
func ParseEnvDir(directory string) ([]string, error) {
	dir, err := ioutil.ReadDir(directory)
	if err != nil {
		return []string{}, err
	}

	lines := []string{}
	for _, info := range dir {
		variable := info.Name()
		// ignore directories and hidden files
		if !info.IsDir() && variable[0] != '.' {
			if !EnvironmentVariableRegexp.MatchString(variable) {
				return []string{}, ErrBadEnvVariable{fmt.Sprintf("variable '%s' is not a valid environment variable", variable)}
			}

			data, err := getFirstLine(filepath.Join(directory, variable))
			if err != nil {
				return []string{}, err
			}

			lines = append(lines, fmt.Sprintf("%s=%s", variable, data))

		}
	}
	return lines, nil
}

func getFirstLine(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return []byte{}, err
	}
	defer file.Close()

	data, _, err := bufio.NewReader(file).ReadLine()
	if err == io.EOF {
		// empty files are OK, they just yield no data.
		// NOTE: this is different behavior from `envdir.c`
		// `envdir.c` will utilize a 0-byte file to indicate
		// that a variable should be removed.
		return []byte{}, nil
	}

	if err != nil {
		return []byte{}, err
	}

	// null bytes are to be converted to newlines according to envdir
	return bytes.TrimSpace(bytes.Replace(data, []byte{0}, []byte{'\n'}, -1)), nil
}
