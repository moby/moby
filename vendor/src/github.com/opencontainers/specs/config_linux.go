package specs

// LinuxSpec is the full specification for linux containers.
type LinuxSpec struct {
	Spec
	// Linux is platform specific configuration for linux based containers.
	Linux Linux `json:"linux"`
}

// Linux contains platform specific configuration for linux based containers.
type Linux struct {
	// Capabilities are linux capabilities that are kept for the container.
	Capabilities []string `json:"capabilities"`
}

// User specifies linux specific user and group information for the container's
// main process.
type User struct {
	// UID is the user id.
	UID uint32 `json:"uid"`
	// GID is the group id.
	GID uint32 `json:"gid"`
	// AdditionalGids are additional group ids set for the container's process.
	AdditionalGids []uint32 `json:"additionalGids"`
}
