package dockerfile

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/docker/pkg/system"
)

var pattern = regexp.MustCompile(`^[a-zA-Z]:\.$`)

// normalizeWorkdir normalizes a user requested working directory in a
// platform semantically consistent way.
func normalizeWorkdir(platform string, current string, requested string) (string, error) {
	if platform == "" {
		platform = "windows"
	}
	if platform == "windows" {
		return normalizeWorkdirWindows(current, requested)
	}
	return normalizeWorkdirUnix(current, requested)
}

// normalizeWorkdirUnix normalizes a user requested working directory in a
// platform semantically consistent way.
func normalizeWorkdirUnix(current string, requested string) (string, error) {
	if requested == "" {
		return "", errors.New("cannot normalize nothing")
	}
	current = strings.Replace(current, string(os.PathSeparator), "/", -1)
	requested = strings.Replace(requested, string(os.PathSeparator), "/", -1)
	if !path.IsAbs(requested) {
		return path.Join(`/`, current, requested), nil
	}
	return requested, nil
}

// normalizeWorkdirWindows normalizes a user requested working directory in a
// platform semantically consistent way.
func normalizeWorkdirWindows(current string, requested string) (string, error) {
	if requested == "" {
		return "", errors.New("cannot normalize nothing")
	}

	// `filepath.Clean` will replace "" with "." so skip in that case
	if current != "" {
		current = filepath.Clean(current)
	}
	if requested != "" {
		requested = filepath.Clean(requested)
	}

	// If either current or requested in Windows is:
	// C:
	// C:.
	// then an error will be thrown as the definition for the above
	// refers to `current directory on drive C:`
	// Since filepath.Clean() will automatically normalize the above
	// to `C:.`, we only need to check the last format
	if pattern.MatchString(current) {
		return "", fmt.Errorf("%s is not a directory. If you are specifying a drive letter, please add a trailing '\\'", current)
	}
	if pattern.MatchString(requested) {
		return "", fmt.Errorf("%s is not a directory. If you are specifying a drive letter, please add a trailing '\\'", requested)
	}

	// Target semantics is C:\somefolder, specifically in the format:
	// UPPERCASEDriveLetter-Colon-Backslash-FolderName. We are already
	// guaranteed that `current`, if set, is consistent. This allows us to
	// cope correctly with any of the following in a Dockerfile:
	//	WORKDIR a                       --> C:\a
	//	WORKDIR c:\\foo                 --> C:\foo
	//	WORKDIR \\foo                   --> C:\foo
	//	WORKDIR /foo                    --> C:\foo
	//	WORKDIR c:\\foo \ WORKDIR bar   --> C:\foo --> C:\foo\bar
	//	WORKDIR C:/foo \ WORKDIR bar    --> C:\foo --> C:\foo\bar
	//	WORKDIR C:/foo \ WORKDIR \\bar  --> C:\foo --> C:\bar
	//	WORKDIR /foo \ WORKDIR c:/bar   --> C:\foo --> C:\bar
	if len(current) == 0 || system.IsAbs(requested) {
		if (requested[0] == os.PathSeparator) ||
			(len(requested) > 1 && string(requested[1]) != ":") ||
			(len(requested) == 1) {
			requested = filepath.Join(`C:\`, requested)
		}
	} else {
		requested = filepath.Join(current, requested)
	}
	// Upper-case drive letter
	return (strings.ToUpper(string(requested[0])) + requested[1:]), nil
}

// equalEnvKeys compare two strings and returns true if they are equal. On
// Windows this comparison is case insensitive.
func equalEnvKeys(from, to string) bool {
	return strings.ToUpper(from) == strings.ToUpper(to)
}
