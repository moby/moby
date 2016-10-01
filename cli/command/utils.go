package command

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// CopyToFile writes the content of the reader to the specified file
func CopyToFile(outfile string, r io.Reader) error {
	tmpFile, err := ioutil.TempFile(filepath.Dir(outfile), ".docker_temp_")
	if err != nil {
		return err
	}

	tmpPath := tmpFile.Name()

	_, err = io.Copy(tmpFile, r)
	tmpFile.Close()

	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err = os.Rename(tmpPath, outfile); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

// capitalizeFirst capitalizes the first character of string
func capitalizeFirst(s string) string {
	switch l := len(s); l {
	case 0:
		return s
	case 1:
		return strings.ToLower(s)
	default:
		return strings.ToUpper(string(s[0])) + strings.ToLower(s[1:])
	}
}

// PrettyPrint outputs arbitrary data for human formatted output by uppercasing the first letter.
func PrettyPrint(i interface{}) string {
	switch t := i.(type) {
	case nil:
		return "None"
	case string:
		return capitalizeFirst(t)
	default:
		return capitalizeFirst(fmt.Sprintf("%s", t))
	}
}

// PromptForConfirmation request and check confirmation from user.
// This will display the provided message followed by ' [y/N] '. If
// the user input 'y' or 'Y' it returns true other false.  If no
// message is provided "Are you sure you want to proceeed? [y/N] "
// will be used instead.
func PromptForConfirmation(ins *InStream, outs *OutStream, message string) bool {
	if message == "" {
		message = "Are you sure you want to proceeed?"
	}
	message += " [y/N] "

	fmt.Fprintf(outs, message)

	answer := ""
	n, _ := fmt.Fscan(ins, &answer)
	if n != 1 || (answer != "y" && answer != "Y") {
		return false
	}

	return true
}
