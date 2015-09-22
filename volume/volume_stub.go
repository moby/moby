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

// FixUIDGID recursively chown the content on container
// creation iff user specifies :u on volume mount and --user at the
// same time.
func FixUIDGID(userID, src, modes string) error {
	return nil
}
