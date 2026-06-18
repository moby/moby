package aws

// RestrictFilePermissions controls whether the SDK restricts file permissions
// on credential cache files it creates.
type RestrictFilePermissions string

const (
	// RestrictFilePermissionsUnset indicates the setting has not been
	// configured.
	RestrictFilePermissionsUnset RestrictFilePermissions = ""

	// RestrictFilePermissionsUserReadWrite sets file permissions to owner
	// read/write only (0600) and directory permissions to owner only (0700)
	// when creating new cache files and directories on Unix. This is the
	// default behavior.
	RestrictFilePermissionsUserReadWrite RestrictFilePermissions = "user_read_write"

	// RestrictFilePermissionsUnrestricted does not set any file or directory
	// permissions, relying on the system's default umask.
	RestrictFilePermissionsUnrestricted RestrictFilePermissions = "unrestricted"
)
