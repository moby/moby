package system // import "github.com/docker/docker/pkg/system"

const defaultUnixPathEnv = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

// DefaultPathEnv is unix style list of directories to search for
// executables. Each directory is separated from the next by a colon
// ':' character .
// For Windows containers, an empty string is returned as the default
// path will be set by the container, and Docker has no context of what the
// default path should be.
func DefaultPathEnv(os string) string {
	if os == "windows" {
		return ""
	}
	return defaultUnixPathEnv
}

// CheckSystemDriveAndRemoveDriveLetter verifies that a path, if it includes a drive letter,
// is the system drive.
// On Linux: this is a no-op.
// On Windows: this does the following>
// CheckSystemDriveAndRemoveDriveLetter verifies and manipulates a Windows path.
// This is used, for example, when validating a user provided path in docker cp.
// If a drive letter is supplied, it must be the system drive. The drive letter
// is always removed. Also, it translates it to OS semantics (IOW / to \). We
// need the path in this syntax so that it can ultimately be concatenated with
// a Windows long-path which doesn't support drive-letters. Examples:
// C:			--> Fail
// C:\			--> \
// a			--> a
// /a			--> \a
// d:\			--> Fail
func CheckSystemDriveAndRemoveDriveLetter(path string) (string, error) {
	return checkSystemDriveAndRemoveDriveLetter(path)
}
