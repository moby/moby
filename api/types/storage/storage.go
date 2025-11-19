// Code generated from OpenAPI definition. DO NOT EDIT.

package storage

// Storage Information about the storage used by the container.
type Storage struct {
	// Information about the storage used for the container's root filesystem.
	//
	RootFS *RootFSStorage `json:"RootFS,omitempty"`
}
