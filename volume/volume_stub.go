// +build !experimental

package volume

// read-write modes
var rwModes = map[string]bool{
	"rw":   true,
	"rw,Z": true,
	"rw,z": true,
	"z,rw": true,
	"Z,rw": true,
	"Z":    true,
	"z":    true,
}

// read-only modes
var roModes = map[string]bool{
	"ro":   true,
	"ro,Z": true,
	"ro,z": true,
	"z,ro": true,
	"Z,ro": true,
}

// ValidMountMode will make sure the mount mode is valid.
// returns if it's a valid mount mode or not.
func ValidMountMode(mode string) bool {
	return roModes[mode] || rwModes[mode]
}

// ReadWrite tells you if a mode string is a valid read-write mode or not.
func ReadWrite(mode string) bool {
	return rwModes[mode]
}

// FixUIDGID recursively chown the content on container
// creation iff user specifies :u on volume mount and --user at the
// same time.
func FixUIDGID(userID, src, modes string) error {
	return nil
}
