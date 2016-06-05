package dockerfile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/docker/pkg/system"
)

// normaliseWorkdir normalises a user requested working directory in a
// platform sematically consistent way.
func normaliseWorkdir(current string, requested string) (string, error) {
	if requested == "" {
		return "", fmt.Errorf("cannot normalise nothing")
	}

	current = filepath.FromSlash(current)
	requested = filepath.FromSlash(requested)

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

func errNotJSON(command, original string) error {
	// For Windows users, give a hint if it looks like it might contain
	// a path which hasn't been escaped such as ["c:\windows\system32\prog.exe", "-param"],
	// as JSON must be escaped. Unfortunate...
	//
	// Specifically looking for quote-driveletter-colon-backslash, there's no
	// double backslash and a [] pair. No, this is not perfect, but it doesn't
	// have to be. It's simply a hint to make life a little easier.
	extra := ""
	original = filepath.FromSlash(strings.ToLower(strings.Replace(strings.ToLower(original), strings.ToLower(command)+" ", "", -1)))
	if len(regexp.MustCompile(`"[a-z]:\\.*`).FindStringSubmatch(original)) > 0 &&
		!strings.Contains(original, `\\`) &&
		strings.Contains(original, "[") &&
		strings.Contains(original, "]") {
		extra = fmt.Sprintf(`. It looks like '%s' includes a file path without an escaped back-slash. JSON requires back-slashes to be escaped such as ["c:\\path\\to\\file.exe", "/parameter"]`, original)
	}
	return fmt.Errorf("%s requires the arguments to be in JSON form%s", command, extra)
}
