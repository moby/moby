// Code generated from OpenAPI definition. DO NOT EDIT.

package container

// FilesystemChange Change in the container's filesystem.
type FilesystemChange struct {
	// Kind of change
	//
	// Can be one of:
	//
	// - `0`: Modified ("C")
	// - `1`: Added ("A")
	// - `2`: Deleted ("D")
	//
	// Required: true
	// Enum: : [0, 1, 2]
	Kind ChangeType `json:"Kind"`

	// Path to file or directory that has changed.
	//
	// Required: true
	Path string `json:"Path"`
}
