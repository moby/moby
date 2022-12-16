package system

const defaultUnixPathEnv = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

// DefaultPathEnv is unix style list of directories to search for
// executables. Each directory is separated from the next by a colon
// ':' character .
// For Windows containers, an empty string is returned as the default
// path will be set by the container, and Docker has no context of what the
// default path should be.
//
// Deprecated: use oci.DefaultPathEnv
func DefaultPathEnv(os string) string {
	if os == "windows" {
		return ""
	}
	return defaultUnixPathEnv
}
