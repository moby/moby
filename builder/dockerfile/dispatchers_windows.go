package dockerfile

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/pkg/system"
)

var pattern = regexp.MustCompile(`^[a-zA-Z]:\.$`)

// normaliseWorkdir normalises a user requested working directory in a
// platform sematically consistent way.
func normaliseWorkdir(current string, requested string) (string, error) {
	if requested == "" {
		return "", fmt.Errorf("cannot normalise nothing")
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

// GETENV
//
// GETENV gets the environment variables from the container to synchronise
// back to the image configuration.
//
func getenv(b *Builder, args []string, attributes map[string]bool, original string) error {
	if len(args) != 1 {
		return errExactlyOneArgument("GETENV")
	}
	if b.image == "" && !b.noBaseImage {
		return fmt.Errorf("Please provide a source image with `from` prior to getenv")
	}
	if err := b.flags.Parse(); err != nil {
		return err
	}

	// Construct a command that gets the value of the requested environment variable in the
	// container, or an empty string if it is not defined. This takes some explaining:
	//
	// 1. Don't use powershell in case it is an optional component in the base image and not installed.
	//
	// 2. Use delayed expansion and ! syntax to pick up variables with newlines in them. Note the only time a "b" appears in the output:
	//    PS C:\> $env:FOO="a`nb"
	//    PS C:\> cmd /v:on /s /c "echo !FOO! && echo %FOO%"; echo "---"; cmd /v:off /s /c "echo !FOO! && echo %FOO%"
	//    a
	//    b
	//    a
	//    ---
	//    !FOO!
	//    a
	//
	// 3. Use command extensions to ensure defined works, otherwise the echo would error rather than return an empty value
	//    PS C:\> cmd /e:on /s /c "if defined PATH (echo hello)"
	//    hello
	//    C:\> cmd /e:off /s /c "if defined PATH (echo hello)"
	//    PATH was unexpected at this time.
	//
	// 4. Don't use `set PATH` as that returns anything starting PATH, which include PATHEXT. Avoids string parsing.
	b.runConfig.Cmd = strslice.StrSlice([]string{"cmd", "/v:on", "/e:on", "/s", "/c", fmt.Sprintf("if defined %s (echo !%s!)", args[0], args[0])})

	cID, err := b.create()
	if err != nil {
		return err
	}

	// Setup stdout and stderr so that we grab the output directly, not back to client.
	origStdout := b.Stdout
	origStderr := b.Stderr
	defer func(o, e io.Writer) {
		b.Stdout = o
		b.Stderr = e
	}(origStdout, origStderr)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	b.Stdout = stdout
	b.Stderr = stderr

	if err := b.run(cID); err != nil {
		return fmt.Errorf("%v - %s", err, string(stderr.Bytes()))
	}

	// After stripping trailing CRLF, treat stdout as the value of the variable and handle the rest as a regular ENV statement
	return env(b, []string{args[0], strings.TrimRight(string(stdout.Bytes()), "\r\n")}, attributes, original)
}
