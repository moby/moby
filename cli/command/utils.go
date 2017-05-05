package command

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/pkg/system"
)

// CopyToFile writes the content of the reader to the specified file
func CopyToFile(outfile string, r io.Reader) error {
	// We use sequential file access here to avoid depleting the standby list
	// on Windows. On Linux, this is a call directly to ioutil.TempFile
	tmpFile, err := system.TempFileSequential(filepath.Dir(outfile), ".docker_temp_")
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

// PromptForConfirmation requests and checks confirmation from user.
// This will display the provided message followed by ' [y/N] '. If
// the user input 'y' or 'Y' it returns true other false.  If no
// message is provided "Are you sure you want to proceed? [y/N] "
// will be used instead.
func PromptForConfirmation(ins *InStream, outs *OutStream, message string) bool {
	if message == "" {
		message = "Are you sure you want to proceed?"
	}
	message += " [y/N] "

	fmt.Fprintf(outs, message)

	// On Windows, force the use of the regular OS stdin stream.
	if runtime.GOOS == "windows" {
		ins = NewInStream(os.Stdin)
	}

	reader := bufio.NewReader(ins)
	answer, _, _ := reader.ReadLine()
	return strings.ToLower(string(answer)) == "y"
}

// PruneFilters returns consolidated prune filters obtained from config.json and cli
func PruneFilters(dockerCli Cli, pruneFilters filters.Args) filters.Args {
	if dockerCli.ConfigFile() == nil {
		return pruneFilters
	}
	for _, f := range dockerCli.ConfigFile().PruneFilters {
		parts := strings.SplitN(f, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] == "label" {
			// CLI label filter supersede config.json.
			// If CLI label filter conflict with config.json,
			// skip adding label! filter in config.json.
			if pruneFilters.Include("label!") && pruneFilters.ExactMatch("label!", parts[1]) {
				continue
			}
		} else if parts[0] == "label!" {
			// CLI label! filter supersede config.json.
			// If CLI label! filter conflict with config.json,
			// skip adding label filter in config.json.
			if pruneFilters.Include("label") && pruneFilters.ExactMatch("label", parts[1]) {
				continue
			}
		}
		pruneFilters.Add(parts[0], parts[1])
	}

	return pruneFilters
}
